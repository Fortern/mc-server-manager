package main

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"log/slog"
	"mc-server-manager/qq"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var (
	//go:embed config-internal.yaml
	embeddedConfig []byte
	v              *viper.Viper
	config         AppConfig
)

type AppConfig struct {
	Server   ServerConfig   `mapstructure:"server"`
	Database DatabaseConfig `mapstructure:"database"`
}

type DatasourceConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
	Database string `mapstructure:"database"`
	Schema   string `mapstructure:"schema"`
}

type DatabaseConfig struct {
	McServer DatasourceConfig `mapstructure:"mc_server"`
	QQBot    DatasourceConfig `mapstructure:"qq_bot"`
}

type ServerConfig struct {
	Port int `mapstructure:"port"`
}

func main() {
	if err := initConfig(); err != nil {
		slog.Error("init config err", "err", err)
		return
	}

	if db, err := initDB(); err != nil {
		slog.Error("init db err", "err", err)
		return
	} else {
		qq.InitBot(db)
	}

	router := gin.Default()
	// ping
	router.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "pong"})
	})

	// QQ Bot
	router.POST("/qq", func(c *gin.Context) {
		appid := c.GetHeader("X-Bot-Appid")
		slog.Info("Get HttpRequest", "appid", appid)
		var payload qq.Payload
		if err := c.ShouldBindJSON(&payload); err != nil {
			slog.Error("JSON parse error.", "err", err)
			return
		}
		switch payload.OPCode {
		case qq.Validate:
			if len(payload.Data) > 0 {
				var validation qq.ValidationPayload
				err := json.Unmarshal(payload.Data, &validation)
				if err != nil {
					slog.Error("JSON parse error.", "err", err)
					return
				}
				slog.Info("Validation Payload",
					"payload.EventTS", validation.EventTS,
					"payload.PlainToken", validation.PlainToken,
				)
				sign, err := qq.GetBotById(appid).Sign(validation)
				if err != nil {
					slog.Error("Signature verification failed.", "err", err)
					return
				}
				c.JSON(http.StatusOK, gin.H{
					"plain_token": validation.PlainToken,
					"signature":   sign,
				})
			}
		}
		c.JSON(http.StatusOK, nil)
		return
	})

	err := router.Run(fmt.Sprintf(":%d", config.Server.Port))
	if err != nil {
		slog.Error("Error starting server", "msg", err)
		return
	}
}

func initDB() (*gorm.DB, error) {
	dataSource := config.Database.QQBot
	url := fmt.Sprintf("postgresql://%s:%s@%s:%d/%s?search_path=%s",
		dataSource.Username,
		dataSource.Password,
		dataSource.Host,
		dataSource.Port,
		dataSource.Database,
		dataSource.Schema,
	)

	if db, err := gorm.Open(postgres.Open(url), &gorm.Config{}); err != nil {
		return nil, err
	} else {
		return db, nil
	}
}

func initConfig() error {
	v = viper.New()
	v.SetConfigType("yaml")

	// read internal config
	if err := v.ReadConfig(bytes.NewReader(embeddedConfig)); err != nil {
		return err
	}

	// create or merge external config
	external := "./manager.yaml"
	if _, err := os.Stat(external); err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		err = os.WriteFile(external, embeddedConfig, 0644)
		if err != nil {
			return err
		}
	}

	v.SetConfigFile(external)
	if err := v.MergeInConfig(); err != nil {
		return err
	}
	if err := v.Unmarshal(&config); err != nil {
		return err
	}
	return nil
}
