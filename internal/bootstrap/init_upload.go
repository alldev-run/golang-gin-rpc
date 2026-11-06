package bootstrap

import (
	"fmt"

	"github.com/alldev-run/golang-gin-rpc/pkg/logger"
	"github.com/alldev-run/golang-gin-rpc/plugins/upload"
)

// InitializeUpload initializes upload service if enabled.
// upload 配置由本组件自行解析（配置结构不再挂在全局 GlobalConfig 上）。
func (b *Bootstrap) InitializeUpload() error {
	var uploadCfg struct {
		Enabled bool          `yaml:"enabled"`
		Config  upload.Config `yaml:",inline"`
	}
	found, err := b.loader.UnmarshalSection("upload", &uploadCfg)
	if err != nil {
		return fmt.Errorf("failed to parse upload config: %w", err)
	}
	if !found || !uploadCfg.Enabled {
		logger.Info("Upload service disabled, skipping initialization")
		return nil
	}

	uploader := upload.NewUploader(&uploadCfg.Config)
	b.uploader = uploader
	b.setDependency(DepUploader, uploader)

	logger.Info("Upload service initialized successfully",
		logger.String("upload_dir", uploadCfg.Config.UploadDir),
		logger.Bool("auto_create_dir", uploadCfg.Config.AutoCreateDir))
	return nil
}
