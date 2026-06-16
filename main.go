package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"mc-server-manager/qq"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var (
	config *viper.Viper
)

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

	err := router.Run(":8070")
	if err != nil {
		slog.Error("Error starting server", "msg", err)
		return
	}
}

func initDB() (*gorm.DB, error) {
	sub := config.Sub("db.qq-bot")
	url := fmt.Sprintf("postgresql://%s:%s@%s:%s/%s?search_path=%s",
		sub.GetString("username"),
		sub.GetString("password"),
		sub.GetString("host"),
		sub.GetString("port"),
		sub.GetString("database"),
		sub.GetString("schema"),
	)

	if db, err := gorm.Open(postgres.Open(url), &gorm.Config{}); err != nil {
		return nil, err
	} else {
		return db, nil
	}
}

func initConfig() error {
	config = viper.New()

	config.SetConfigName("manager")
	config.SetConfigType("yaml")
	config.AddConfigPath("/etc/mc-server-manager/")
	config.AddConfigPath("./config/")

	if err := config.ReadInConfig(); err != nil {
		return err
	}

	db1 := config.Sub("db.mc-server")
	db2 := config.Sub("db.qq-bot")

	db1.Set("host", "localhost")
	db1.Set("port", 5432)
	db1.Set("username", "postgres")
	db1.Set("password", "postgres")
	db1.Set("dbname", "mc_server")
	db1.Set("schema", "my_server")

	db2.Set("host", "localhost")
	db2.Set("port", 5432)
	db2.Set("username", "postgres")
	db2.Set("password", "postgres")
	db2.Set("dbname", "qq-bot")
	db1.Set("schema", "my_server")

	return nil
}
