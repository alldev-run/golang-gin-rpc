package main

import (
	"bufio"
	"context"
	"database/sql"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"sort"
	"strings"
	"time"

	"github.com/alldev-run/golang-gin-rpc/plugins/db/migration"
	_ "github.com/go-sql-driver/mysql"
)

func main() {
	if len(os.Args) < 2 {
		fatalf("usage: scaffold <command> [args]\ncommands: init, create-api, clean-logs, export-template, gen-migration, run-migration")
	}

	switch os.Args[1] {
	case "init":
		initApp(os.Args[2:])
	case "create-api":
		createAPI(os.Args[2:])
	case "clean-logs":
		cleanLogs(os.Args[2:])
	case "export-template":
		exportTemplate(os.Args[2:])
	case "gen-migration":
		genMigration(os.Args[2:])
	case "run-migration":
		runMigration(os.Args[2:])
	default:
		fatalf("unknown command: %s", os.Args[1])
	}
}

type sqlMigrationSpec struct {
	Version int
	Name    string
	UpSQL   string
	DownSQL string
}

func runMigration(args []string) {
	fsFlag := flag.NewFlagSet("run-migration", flag.ExitOnError)
	dirFlag := fsFlag.String("dir", "migrations", "migration files directory")
	driverFlag := fsFlag.String("driver", "mysql", "database driver (e.g. mysql)")
	dsnFlag := fsFlag.String("dsn", "", "database DSN")
	actionFlag := fsFlag.String("action", "up", "action: up|down|status")
	stepsFlag := fsFlag.Int("steps", 1, "rollback steps for action=down")
	tableFlag := fsFlag.String("table", "schema_migrations", "migration tracking table name")
	timeoutFlag := fsFlag.Duration("timeout", 30*time.Second, "operation timeout")
	_ = fsFlag.Parse(args)

	if strings.TrimSpace(*dsnFlag) == "" {
		fatalf("missing --dsn")
	}

	action := strings.ToLower(strings.TrimSpace(*actionFlag))
	if action != "up" && action != "down" && action != "status" {
		fatalf("invalid --action: %s (allowed: up|down|status)", action)
	}

	root, err := os.Getwd()
	if err != nil {
		fatalf("getwd: %v", err)
	}

	dir := strings.TrimSpace(*dirFlag)
	if dir == "" {
		dir = "migrations"
	}
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(root, dir)
	}

	specs, err := loadSQLMigrations(dir)
	if err != nil {
		fatalf("load migrations: %v", err)
	}

	db, err := sql.Open(strings.TrimSpace(*driverFlag), strings.TrimSpace(*dsnFlag))
	if err != nil {
		fatalf("open db: %v", err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), *timeoutFlag)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		fatalf("ping db: %v", err)
	}

	m := migration.New(db)
	tableName := strings.TrimSpace(*tableFlag)
	if tableName != "" {
		m.TableName = tableName
	}
	for _, spec := range specs {
		m.Add(spec.Version, spec.Name, spec.UpSQL, spec.DownSQL)
	}

	switch action {
	case "up":
		if err := m.Up(ctx); err != nil {
			fatalf("run up migrations: %v", err)
		}
		fmt.Println("migrations applied successfully")
	case "down":
		steps := *stepsFlag
		if steps <= 0 {
			steps = 1
		}
		for i := 0; i < steps; i++ {
			if err := m.Down(ctx); err != nil {
				fatalf("run down migration (step %d): %v", i+1, err)
			}
		}
		fmt.Printf("rollback completed, steps=%d\n", steps)
	case "status":
		statuses, err := m.Status(ctx)
		if err != nil {
			fatalf("query migration status: %v", err)
		}
		if len(statuses) == 0 {
			fmt.Println("no migrations registered")
			return
		}
		for _, s := range statuses {
			fmt.Printf("%d\t%s\tapplied=%v\n", s.Version, s.Name, s.Applied)
		}
	}
}

func loadSQLMigrations(dir string) ([]sqlMigrationSpec, error) {
	if stat, err := os.Stat(dir); err != nil || !stat.IsDir() {
		return nil, fmt.Errorf("migration dir not found: %s", dir)
	}

	re := regexp.MustCompile(`^(\d+)_([a-z0-9_]+)\.(up|down)\.sql$`)
	type holder struct {
		Version int
		Name    string
		UpPath  string
		DownPath string
	}
	items := make(map[string]*holder)

	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		base := d.Name()
		match := re.FindStringSubmatch(base)
		if len(match) == 0 {
			return nil
		}

		version, convErr := strconv.Atoi(match[1])
		if convErr != nil {
			return fmt.Errorf("invalid migration version in %s: %w", base, convErr)
		}
		name := match[2]
		direction := match[3]
		key := match[1] + "_" + name

		h := items[key]
		if h == nil {
			h = &holder{Version: version, Name: name}
			items[key] = h
		}

		if direction == "up" {
			h.UpPath = path
		} else {
			h.DownPath = path
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	if len(items) == 0 {
		return nil, fmt.Errorf("no migration files found in %s", dir)
	}

	specs := make([]sqlMigrationSpec, 0, len(items))
	for _, h := range items {
		if h.UpPath == "" || h.DownPath == "" {
			return nil, fmt.Errorf("migration %d_%s missing up/down pair", h.Version, h.Name)
		}
		upBytes, err := os.ReadFile(h.UpPath)
		if err != nil {
			return nil, fmt.Errorf("read up sql %s: %w", h.UpPath, err)
		}
		downBytes, err := os.ReadFile(h.DownPath)
		if err != nil {
			return nil, fmt.Errorf("read down sql %s: %w", h.DownPath, err)
		}
		specs = append(specs, sqlMigrationSpec{
			Version: h.Version,
			Name:    h.Name,
			UpSQL:   string(upBytes),
			DownSQL: string(downBytes),
		})
	}

	sort.Slice(specs, func(i, j int) bool {
		return specs[i].Version < specs[j].Version
	})

	return specs, nil
}

func genMigration(args []string) {
	fsFlag := flag.NewFlagSet("gen-migration", flag.ExitOnError)
	nameFlag := fsFlag.String("name", "", "migration name, e.g. create_users_table")
	dirFlag := fsFlag.String("dir", "migrations", "output directory for migration files")
	versionFlag := fsFlag.String("version", "", "migration version prefix (default: UTC timestamp YYYYMMDDHHMMSS)")
	_ = fsFlag.Parse(args)

	name := strings.TrimSpace(*nameFlag)
	if name == "" && fsFlag.NArg() > 0 {
		name = strings.TrimSpace(strings.Join(fsFlag.Args(), "_"))
	}
	if name == "" {
		fatalf("missing migration name, use: scaffold gen-migration --name create_users_table")
	}

	name = sanitizeMigrationName(name)
	if name == "" {
		fatalf("invalid migration name")
	}

	version := strings.TrimSpace(*versionFlag)
	if version == "" {
		version = time.Now().UTC().Format("20060102150405")
	}

	root, err := os.Getwd()
	if err != nil {
		fatalf("getwd: %v", err)
	}

	outDir := strings.TrimSpace(*dirFlag)
	if outDir == "" {
		outDir = "migrations"
	}
	if !filepath.IsAbs(outDir) {
		outDir = filepath.Join(root, outDir)
	}

	if err := os.MkdirAll(outDir, 0o755); err != nil {
		fatalf("create migration directory: %v", err)
	}

	base := version + "_" + name
	upPath := filepath.Join(outDir, base+".up.sql")
	downPath := filepath.Join(outDir, base+".down.sql")

	if _, err := os.Stat(upPath); err == nil {
		fatalf("migration already exists: %s", upPath)
	}
	if _, err := os.Stat(downPath); err == nil {
		fatalf("migration already exists: %s", downPath)
	}

	upContent := fmt.Sprintf("-- Migration: %s\n-- Version: %s\n\n-- Write your UP SQL here\n", name, version)
	downContent := fmt.Sprintf("-- Rollback: %s\n-- Version: %s\n\n-- Write your DOWN SQL here\n", name, version)

	if err := os.WriteFile(upPath, []byte(upContent), 0o644); err != nil {
		fatalf("write up migration: %v", err)
	}
	if err := os.WriteFile(downPath, []byte(downContent), 0o644); err != nil {
		_ = os.Remove(upPath)
		fatalf("write down migration: %v", err)
	}

	fmt.Printf("created migration:\n- %s\n- %s\n", upPath, downPath)
}

func sanitizeMigrationName(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	name = strings.ReplaceAll(name, "-", "_")
	name = strings.ReplaceAll(name, " ", "_")

	var b strings.Builder
	lastUnderscore := false
	for _, ch := range name {
		isLetter := ch >= 'a' && ch <= 'z'
		isDigit := ch >= '0' && ch <= '9'
		if isLetter || isDigit {
			b.WriteRune(ch)
			lastUnderscore = false
			continue
		}
		if !lastUnderscore {
			b.WriteRune('_')
			lastUnderscore = true
		}
	}

	cleaned := strings.Trim(b.String(), "_")
	for strings.Contains(cleaned, "__") {
		cleaned = strings.ReplaceAll(cleaned, "__", "_")
	}
	return cleaned
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

	localSrc := filepath.Join(projectRoot, "plugins", "gateway", "templates", template)
	if stat, err := os.Stat(localSrc); err == nil && stat.IsDir() {
		return localSrc, nil
	}

	frameworkRoots := queryFrameworkRoots(projectRoot)
	for _, frameworkRoot := range frameworkRoots {
		moduleSrc := filepath.Join(frameworkRoot, "plugins", "gateway", "templates", template)
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
			if strings.HasSuffix(base, "_example.go.gotmpl") {
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
	template := fsFlag.String("template", "http-gateway", "template name under ./plugins/gateway/templates")
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
	dst := filepath.Join(root, "plugins", "gateway", "templates", *template)

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

func cleanLogs(args []string) {
	fsFlag := flag.NewFlagSet("clean-logs", flag.ExitOnError)
	dirFlag := fsFlag.String("dir", "logs", "log directory path")
	daysFlag := fsFlag.Int("days", 7, "retention days, delete files older than N days")
	dryRunFlag := fsFlag.Bool("dry-run", false, "preview files to delete without removing them")
	patternFlag := fsFlag.String("pattern", "*.log", "file pattern to match (e.g., *.log, *.txt)")
	recursiveFlag := fsFlag.Bool("recursive", false, "clean subdirectories recursively")
	_ = fsFlag.Parse(args)

	root, err := os.Getwd()
	if err != nil {
		fatalf("getwd: %v", err)
	}

	logDir := strings.TrimSpace(*dirFlag)
	if logDir == "" {
		logDir = "logs"
	}
	if !filepath.IsAbs(logDir) {
		logDir = filepath.Join(root, logDir)
	}

	stat, err := os.Stat(logDir)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Printf("log directory not found: %s\n", logDir)
			return
		}
		fatalf("stat log dir: %v", err)
	}
	if !stat.IsDir() {
		fatalf("not a directory: %s", logDir)
	}

	cutoff := time.Now().AddDate(0, 0, -*daysFlag)
	pattern := *patternFlag
	if pattern == "" {
		pattern = "*.log"
	}

	var filesToDelete []string
	var totalSize int64

	walkFn := func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if !*recursiveFlag && path != logDir {
				return fs.SkipDir
			}
			return nil
		}

		matched, _ := filepath.Match(pattern, d.Name())
		if !matched {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return nil
		}

		if info.ModTime().Before(cutoff) {
			filesToDelete = append(filesToDelete, path)
			totalSize += info.Size()
		}
		return nil
	}

	if err := filepath.WalkDir(logDir, walkFn); err != nil {
		fatalf("walk log dir: %v", err)
	}

	if len(filesToDelete) == 0 {
		fmt.Println("no old log files to clean")
		return
	}

	fmt.Printf("found %d file(s) older than %d days (total size: %s)\n",
		len(filesToDelete), *daysFlag, formatBytes(totalSize))

	if *dryRunFlag {
		fmt.Println("dry-run mode, files to delete:")
		for _, f := range filesToDelete {
			fmt.Printf("  - %s\n", f)
		}
		return
	}

	var deleted int
	var failed int
	var freed int64

	for _, f := range filesToDelete {
		info, _ := os.Stat(f)
		if err := os.Remove(f); err != nil {
			fmt.Printf("failed to delete %s: %v\n", f, err)
			failed++
			continue
		}
		deleted++
		if info != nil {
			freed += info.Size()
		}
	}

	fmt.Printf("cleaned %d file(s), failed %d, freed %s\n", deleted, failed, formatBytes(freed))
}

func formatBytes(n int64) string {
	const (
		KB = 1024
		MB = 1024 * KB
		GB = 1024 * MB
	)
	switch {
	case n >= GB:
		return fmt.Sprintf("%.2f GB", float64(n)/GB)
	case n >= MB:
		return fmt.Sprintf("%.2f MB", float64(n)/MB)
	case n >= KB:
		return fmt.Sprintf("%.2f KB", float64(n)/KB)
	default:
		return fmt.Sprintf("%d B", n)
	}
}

func fatalf(format string, args ...any) {
	_, _ = fmt.Fprintf(os.Stderr, format+"\n", args...)
	exit(1)
}

func exit(code int) {
	os.Exit(code)
}

// ===== scaffold init:一条命令生成应用骨架,按需选择组件 =====

// initComponent describes a selectable component for `scaffold init`.
type initComponent struct {
	name string
	kind string // "service" or "plugin"
	desc string
}

// initCatalog lists all selectable components, services first so the menu groups naturally.
var initCatalog = []initComponent{
	{"api-gateway", "service", "HTTP 网关服务 (gin)"},
	{"rpc", "service", "gRPC/JSON-RPC 托管服务"},
	{"websocket", "service", "WebSocket 托管服务"},
	{"db", "plugin", "数据库 (mysql/postgres/clickhouse)"},
	{"cache", "plugin", "Redis 缓存"},
	{"discovery", "plugin", "服务发现 (consul/etcd/zookeeper/nacos)"},
	{"tracing", "plugin", "链路追踪 (jaeger/otlp)"},
	{"auth", "plugin", "JWT 认证"},
	{"metrics", "plugin", "Prometheus 指标 (:9090)"},
	{"health", "plugin", "健康检查 (db/cache 未启用时自动降级)"},
	{"errors", "plugin", "错误处理"},
	{"upload", "plugin", "文件上传"},
}

// initDefaults mirrors the default-enabled set in configs/config.yaml.
var initDefaults = []string{"api-gateway", "metrics", "health", "errors"}

func initApp(args []string) {
	fsFlag := flag.NewFlagSet("init", flag.ExitOnError)
	name := fsFlag.String("name", "", "app name, e.g. my-app")
	components := fsFlag.String("components", "", "comma separated components (numbers or names); empty = interactive menu, defaults on non-TTY")
	port := fsFlag.Int("port", 8080, "server.http.port in generated config")
	_ = fsFlag.Parse(args)

	if strings.TrimSpace(*name) == "" {
		fatalf("missing --name. example: scaffold init --name my-app --components api-gateway,db,cache")
	}
	if strings.Contains(*name, string(os.PathSeparator)) || strings.Contains(*name, "/") || strings.Contains(*name, "\\") {
		fatalf("invalid --name: %q", *name)
	}
	if *port <= 0 || *port > 65535 {
		fatalf("invalid --port: %d", *port)
	}

	root, err := os.Getwd()
	if err != nil {
		fatalf("getwd: %v", err)
	}
	module, err := readModuleName(filepath.Join(root, "go.mod"))
	if err != nil {
		fatalf("read module from go.mod: %v", err)
	}

	// Resolve selection: flags > interactive menu (TTY) > defaults (non-TTY).
	var sel []string
	switch {
	case strings.TrimSpace(*components) != "":
		sel, err = parseComponents(*components)
	default:
		if stdinIsTTY() {
			sel, err = promptComponents(os.Stdin, os.Stdout)
		} else {
			sel = append([]string(nil), initDefaults...)
			fmt.Printf("non-interactive stdin, using default components: %s\n", strings.Join(sel, ","))
		}
	}
	if err != nil {
		fatalf("%v", err)
	}

	src, err := resolveTemplateSource(root, "app-skeleton")
	if err != nil {
		fatalf("resolve template failed: %v", err)
	}
	dst := filepath.Join(root, "api", *name)
	if _, err := os.Stat(dst); err == nil {
		fatalf("target already exists: %s", dst)
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		fatalf("create api dir: %v", err)
	}

	rollback := func(stage string) {
		_ = os.RemoveAll(dst)
		fatalf("init failed at %s: %v", stage, err)
	}

	tokens := map[string]string{
		"__MODULE__":           module,
		"__APP_NAME__":         *name,
		"__API_PATH__":         filepath.ToSlash(filepath.Join("api", *name)),
		"__EXTRA_IMPORTS__":    buildExtraImports(sel),
		"__SERVICES_LIST__":    buildServicesList(sel),
		"__WEBSOCKET_WIRING__": buildServiceWiring(sel, *port),
	}
	if err := copyTreeWithReplace(src, dst, copyOptions{TemplateDir: true}, func(_ string, data []byte) []byte {
		s := string(data)
		for k, v := range tokens {
			s = strings.ReplaceAll(s, k, v)
		}
		return []byte(s)
	}); err != nil {
		rollback("template copy")
	}

	cfgPath := filepath.Join(dst, "config", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
		rollback("config dir")
	}
	if err := os.WriteFile(cfgPath, buildConfigYAML(sel, *name, *port), 0o644); err != nil {
		rollback("config write")
	}

	fmt.Printf("created: %s\n", dst)
	fmt.Printf("components: %s\n", strings.Join(sel, ","))
	fmt.Printf("next:\n  go build ./api/%s\n  go run ./api/%s\n", *name, *name)
}

func stdinIsTTY() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

// promptComponents prints the numbered menu and reads a single selection line.
func promptComponents(in io.Reader, out io.Writer) ([]string, error) {
	fmt.Fprintln(out, "选择要启用的组件(输入编号或名称,逗号分隔;回车=默认):")
	for i, c := range initCatalog {
		tag := "组件"
		if c.kind == "service" {
			tag = "服务"
		}
		fmt.Fprintf(out, "  [%2d] %-12s %s\n", i+1, c.name, tag+" — "+c.desc)
	}
	fmt.Fprintf(out, "  默认: %s\n> ", strings.Join(initDefaults, ","))

	line, err := bufio.NewReader(in).ReadString('\n')
	if err != nil && strings.TrimSpace(line) == "" {
		return nil, fmt.Errorf("read selection: %w", err)
	}
	return parseSelectionInput(line)
}

// parseSelectionInput accepts numbers (1-based catalog index), names, or both.
func parseSelectionInput(line string) ([]string, error) {
	fields := strings.FieldsFunc(line, func(r rune) bool {
		return r == ',' || r == ' ' || r == '\t' || r == '\n' || r == '\r'
	})
	if len(fields) == 0 {
		return append([]string(nil), initDefaults...), nil
	}
	var parts []string
	for _, f := range fields {
		if n, err := strconv.Atoi(f); err == nil {
			if n < 1 || n > len(initCatalog) {
				return nil, fmt.Errorf("component number out of range: %d", n)
			}
			parts = append(parts, initCatalog[n-1].name)
			continue
		}
		parts = append(parts, f)
	}
	return dedupeValidComponents(parts)
}

// parseComponents parses a --components flag value.
func parseComponents(s string) ([]string, error) {
	fields := strings.Split(s, ",")
	var parts []string
	for _, f := range fields {
		if trimmed := strings.TrimSpace(f); trimmed != "" {
			parts = append(parts, trimmed)
		}
	}
	return dedupeValidComponents(parts)
}

func dedupeValidComponents(parts []string) ([]string, error) {
	seen := make(map[string]bool, len(parts))
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		name := strings.ToLower(p)
		if !isValidInitComponent(name) {
			return nil, fmt.Errorf("unknown component %q; valid: %s", p, strings.Join(initComponentNames(), ", "))
		}
		if seen[name] {
			continue
		}
		seen[name] = true
		out = append(out, name)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no components selected; valid: %s", strings.Join(initComponentNames(), ", "))
	}
	return out, nil
}

func isValidInitComponent(name string) bool {
	for _, c := range initCatalog {
		if c.name == name {
			return true
		}
	}
	return false
}

func initComponentNames() []string {
	names := make([]string, len(initCatalog))
	for i, c := range initCatalog {
		names[i] = c.name
	}
	return names
}

func isSelected(sel []string, name string) bool {
	for _, c := range sel {
		if c == name {
			return true
		}
	}
	return false
}

func initKind(name string) string {
	for _, c := range initCatalog {
		if c.name == name {
			return c.kind
		}
	}
	return ""
}

// buildServicesList renders the Go slice literal for frameworkOptions.Services.
func buildServicesList(sel []string) string {
	var quoted []string
	for _, c := range initCatalog {
		if c.kind == "service" && isSelected(sel, c.name) {
			quoted = append(quoted, strconv.Quote(c.name))
		}
	}
	return strings.Join(quoted, ", ")
}

// buildExtraImports renders additional import lines for the selected services.
// A trailing newline keeps the closing paren on its own line; empty when unused.
func buildExtraImports(sel []string) string {
	if isSelected(sel, "websocket") {
		return "\t\"github.com/alldev-run/golang-gin-rpc/plugins/websocket\"\n"
	}
	return ""
}

// buildServiceWiring renders option wiring for services that need it (websocket).
func buildServiceWiring(sel []string, httpPort int) string {
	if !isSelected(sel, "websocket") {
		return ""
	}
	return fmt.Sprintf(`	// WebSocket 监听地址默认取 http 端口 +1,可在下方调整
	wsCfg := websocket.DefaultServerConfig()
	wsCfg.Addr = ":%d"
	frameworkOptions.WebSocket = &bootstrap.WebSocketServiceOptions{
		Name:   "websocket",
		Config: wsCfg,
	}
`, httpPort+1)
}

// buildConfigYAML generates config.yaml with toggles and sections matching the
// selection. Key shapes follow pkg/config GlobalConfig (server.http.*, database.primary...).
func buildConfigYAML(sel []string, appName string, port int) []byte {
	var b strings.Builder
	b.WriteString("# Generated by scaffold init — 组件开关: plugins.* + framework.services\n")
	fmt.Fprintf(&b, "app:\n  name: %s\n  environment: development\n\n", appName)

	b.WriteString("plugins:\n")
	for _, c := range initCatalog {
		if c.kind != "plugin" {
			continue
		}
		fmt.Fprintf(&b, "  %s: { enabled: %t }\n", c.name, isSelected(sel, c.name))
	}

	quoted := strings.Split(buildServicesList(sel), ", ")
	fmt.Fprintf(&b, "\nframework:\n  services: [%s]\n  validate_dependency_coverage: true\n\n", strings.Join(quoted, ", "))

	fmt.Fprintf(&b, `server:
  http:
    host: "0.0.0.0"
    port: %d
    read_timeout: "30s"
    write_timeout: "30s"
    idle_timeout: "60s"
`, port)

	if isSelected(sel, "db") {
		b.WriteString(`
# 数据库:驱动可换 postgres / clickhouse 等(见 plugins/db)
database:
  primary:
    enabled: true
    driver: mysql
    host: "127.0.0.1"
    port: 3306
    database: app_db
    username: root
    password: ""
    log_enabled: true
`)
	}
	if isSelected(sel, "cache") {
		b.WriteString(`
redis:
  host: "127.0.0.1"
  port: 6379
`)
	}
	if isSelected(sel, "rpc") {
		b.WriteString(`
rpc:
  enabled: true
  protocol: grpc
`)
	}
	if isSelected(sel, "discovery") {
		b.WriteString(`
discovery:
  enabled: true
  type: consul
  endpoints: ["127.0.0.1:8500"]
  namespace: default
`)
	}
	if isSelected(sel, "metrics") || isSelected(sel, "tracing") {
		b.WriteString("\nobservability:\n")
		if isSelected(sel, "metrics") {
			b.WriteString(`  metrics:
    enabled: true
    type: prometheus
    endpoint: ":9090"
    namespace: ` + appName + "\n")
		}
		if isSelected(sel, "tracing") {
			b.WriteString(`  tracing:
    enabled: true
    type: jaeger
    endpoint: "http://127.0.0.1:14268/api/traces"
    sample_rate: 1.0
`)
		}
	}
	if isSelected(sel, "auth") {
		b.WriteString(`
security:
  jwt:
    enabled: true
    secret: "change-me"
    algorithm: HS256
    expiration: 24h
`)
	}
	if isSelected(sel, "upload") {
		b.WriteString(`
upload:
  enabled: true
  upload_dir: ./uploads
  auto_create_dir: true
`)
	}
	return []byte(b.String())
}
