package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

func main() {
	if len(os.Args) < 2 {
		fatalf("usage: scaffold <command> [args]\ncommands: create-api, export-template")
	}

	switch os.Args[1] {
	case "create-api":
		createAPI(os.Args[2:])
	case "export-template":
		exportTemplate(os.Args[2:])
	default:
		fatalf("unknown command: %s", os.Args[1])
	}
}

func createAPI(args []string) {
	fsFlag := flag.NewFlagSet("create-api", flag.ExitOnError)
	name := fsFlag.String("name", "", "new api name, e.g. user-gateway")
	template := fsFlag.String("template", "http-gateway", "template name, e.g. http-gateway")
	_ = fsFlag.Parse(args)

	if strings.TrimSpace(*name) == "" {
		fatalf("missing --name")
	}
	if strings.Contains(*name, string(os.PathSeparator)) || strings.Contains(*name, "/") || strings.Contains(*name, "\\") {
		fatalf("invalid --name: %q", *name)
	}

	root, err := os.Getwd()
	if err != nil {
		fatalf("getwd: %v", err)
	}

	module, err := readModuleName(filepath.Join(root, "go.mod"))
	if err != nil {
		fatalf("read module from go.mod: %v", err)
	}

	src, err := resolveTemplateSource(root, *template)
	if err != nil {
		fatalf("resolve template failed: %v", err)
	}
	dst := filepath.Join(root, "api", *name)

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		fatalf("create api dir: %v", err)
	}

	if _, err := os.Stat(dst); err == nil {
		fatalf("target already exists: %s", dst)
	}

	tokens := map[string]string{
		"__MODULE__":   module,
		"__API_NAME__": *name,
		"__API_PATH__": filepath.ToSlash(filepath.Join("api", *name)),
	}

	if err := copyTreeWithReplace(src, dst, copyOptions{TemplateDir: true}, func(_ string, data []byte) []byte {
		s := string(data)
		for k, v := range tokens {
			s = strings.ReplaceAll(s, k, v)
		}
		return []byte(s)
	}); err != nil {
		_ = os.RemoveAll(dst)
		fatalf("create api failed: %v", err)
	}

	fmt.Printf("created: %s\n", dst)
}

func resolveTemplateSource(projectRoot, template string) (string, error) {
	if strings.TrimSpace(template) == "" {
		return "", fmt.Errorf("template is empty")
	}

	if strings.Contains(template, string(os.PathSeparator)) || strings.Contains(template, "/") || strings.Contains(template, "\\") {
		return "", fmt.Errorf("invalid --template: %q", template)
	}

	if customDir := strings.TrimSpace(os.Getenv("SCAFFOLD_TEMPLATE_DIR")); customDir != "" {
		customSrc := filepath.Join(customDir, template)
		if stat, err := os.Stat(customSrc); err == nil && stat.IsDir() {
			return customSrc, nil
		}
	}

	localSrc := filepath.Join(projectRoot, "pkg", "gateway", "templates", template)
	if stat, err := os.Stat(localSrc); err == nil && stat.IsDir() {
		return localSrc, nil
	}

	frameworkRoots := queryFrameworkRoots(projectRoot)
	for _, frameworkRoot := range frameworkRoots {
		moduleSrc := filepath.Join(frameworkRoot, "pkg", "gateway", "templates", template)
		if stat, err := os.Stat(moduleSrc); err == nil && stat.IsDir() {
			return moduleSrc, nil
		}
	}

	return "", fmt.Errorf("template %q not found (checked local repo, SCAFFOLD_TEMPLATE_DIR, and module cache)", template)
}

func queryFrameworkRoots(projectRoot string) []string {
	seen := make(map[string]struct{})
	var roots []string

	if dir, ok := queryModuleDir(projectRoot, "github.com/alldev-run/golang-gin-rpc"); ok {
		seen[dir] = struct{}{}
		roots = append(roots, dir)
	}

	if dir, ok := queryModuleDir(projectRoot, "github.com/alldev-run/golang-gin-rpc@latest"); ok {
		if _, exists := seen[dir]; !exists {
			seen[dir] = struct{}{}
			roots = append(roots, dir)
		}
	}

	for _, dir := range queryFrameworkRootsFromModCache(projectRoot) {
		if _, exists := seen[dir]; exists {
			continue
		}
		seen[dir] = struct{}{}
		roots = append(roots, dir)
	}

	return roots
}

func queryModuleDir(projectRoot, module string) (string, bool) {
	cmd := exec.Command("go", "list", "-m", "-f", "{{.Dir}}", module)
	cmd.Dir = projectRoot
	out, err := cmd.Output()
	if err != nil {
		return "", false
	}

	dir := strings.TrimSpace(string(out))
	if dir == "" {
		return "", false
	}

	if stat, err := os.Stat(dir); err != nil || !stat.IsDir() {
		return "", false
	}

	return dir, true
}

func queryFrameworkRootsFromModCache(projectRoot string) []string {
	modCache := strings.TrimSpace(os.Getenv("GOMODCACHE"))
	if modCache == "" {
		cmd := exec.Command("go", "env", "GOMODCACHE")
		cmd.Dir = projectRoot
		if out, err := cmd.Output(); err == nil {
			modCache = strings.TrimSpace(string(out))
		}
	}

	if modCache == "" {
		return nil
	}

	pattern := filepath.Join(modCache, "github.com", "alldev-run", "golang-gin-rpc@*")
	matches, err := filepath.Glob(pattern)
	if err != nil || len(matches) == 0 {
		return nil
	}

	sort.Strings(matches)
	for i, j := 0, len(matches)-1; i < j; i, j = i+1, j-1 {
		matches[i], matches[j] = matches[j], matches[i]
	}

	var roots []string
	for _, candidate := range matches {
		if stat, err := os.Stat(candidate); err == nil && stat.IsDir() {
			roots = append(roots, candidate)
		}
	}

	return roots
}

type copyOptions struct {
	TemplateDir bool
}

func copyTreeWithReplace(srcDir, dstDir string, opt copyOptions, replace func(rel string, data []byte) []byte) error {
	return filepath.WalkDir(srcDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return os.MkdirAll(dstDir, 0o755)
		}

		base := d.Name()
		if base == ".git" || base == "bin" {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if opt.TemplateDir {
			if strings.HasSuffix(base, ".go") {
				return nil
			}
		}

		dstRel := rel
		if strings.HasSuffix(dstRel, ".gotmpl") {
			dstRel = strings.TrimSuffix(dstRel, ".gotmpl")
		}
		dstPath := filepath.Join(dstDir, dstRel)
		if d.IsDir() {
			return os.MkdirAll(dstPath, 0o755)
		}

		info, err := d.Info()
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
			return err
		}

		in, err := os.Open(path)
		if err != nil {
			return err
		}
		defer in.Close()

		out, err := os.OpenFile(dstPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode())
		if err != nil {
			return err
		}
		defer func() { _ = out.Close() }()

		data, err := io.ReadAll(in)
		if err != nil {
			return err
		}

		if isBinary(data) {
			_, err = out.Write(data)
			return err
		}

		if replace == nil {
			_, err = out.Write(data)
			return err
		}

		data = replace(filepath.ToSlash(rel), data)
		_, err = out.Write(data)
		return err
	})
}

func isBinary(b []byte) bool {
	for _, c := range b {
		if c == 0 {
			return true
		}
	}
	return false
}

func readModuleName(goModPath string) (string, error) {
	f, err := os.Open(goModPath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	s := bufio.NewScanner(f)
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module ")), nil
		}
	}
	if err := s.Err(); err != nil {
		return "", err
	}
	return "", fmt.Errorf("module directive not found")
}

func exportTemplate(args []string) {
	fsFlag := flag.NewFlagSet("export-template", flag.ExitOnError)
	srcName := fsFlag.String("name", "", "api name under ./api, e.g. http-gateway")
	template := fsFlag.String("template", "http-gateway", "template name under ./pkg/gateway/templates")
	_ = fsFlag.Parse(args)

	if strings.TrimSpace(*srcName) == "" {
		fatalf("missing --name")
	}

	root, err := os.Getwd()
	if err != nil {
		fatalf("getwd: %v", err)
	}

	module, err := readModuleName(filepath.Join(root, "go.mod"))
	if err != nil {
		fatalf("read module from go.mod: %v", err)
	}

	src := filepath.Join(root, "api", *srcName)
	dst := filepath.Join(root, "pkg", "gateway", "templates", *template)

	if _, err := os.Stat(src); err != nil {
		fatalf("source api not found: %s", src)
	}

	apiPath := filepath.ToSlash(filepath.Join("api", *srcName))
	name := *srcName

	if err := os.RemoveAll(dst); err != nil {
		fatalf("clean dst: %v", err)
	}
	if err := os.MkdirAll(dst, 0o755); err != nil {
		fatalf("mkdir dst: %v", err)
	}

	if err := filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}

		base := d.Name()
		if base == ".git" || base == "bin" {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}

		dstRel := rel
		if strings.HasSuffix(dstRel, ".go") {
			dstRel = dstRel + ".gotmpl"
		}
		dstPath := filepath.Join(dst, dstRel)

		if d.IsDir() {
			return os.MkdirAll(dstPath, 0o755)
		}

		info, err := d.Info()
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
			return err
		}

		in, err := os.Open(path)
		if err != nil {
			return err
		}
		defer in.Close()

		out, err := os.OpenFile(dstPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode())
		if err != nil {
			return err
		}
		defer func() { _ = out.Close() }()

		data, err := io.ReadAll(in)
		if err != nil {
			return err
		}

		if isBinary(data) {
			_, err = out.Write(data)
			return err
		}

		s := string(data)
		s = strings.ReplaceAll(s, module, "__MODULE__")
		s = strings.ReplaceAll(s, apiPath, "__API_PATH__")
		s = strings.ReplaceAll(s, name, "__API_NAME__")
		_, err = out.Write([]byte(s))
		return err
	}); err != nil {
		_ = os.RemoveAll(dst)
		fatalf("export template failed: %v", err)
	}

	fmt.Printf("exported template: %s\n", dst)
}

func fatalf(format string, args ...any) {
	_, _ = fmt.Fprintf(os.Stderr, format+"\n", args...)
	exit(1)
}

func exit(code int) {
	os.Exit(code)
}
