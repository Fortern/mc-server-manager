package minecraft

import (
	"context"

	"gorm.io/gorm"
)

// GetPlayer TODO 数据库集成
func GetPlayer(db *gorm.DB) Player {

	ctx := context.Background()
	player, _ := gorm.G[Player](db).Where("id = ?", 10).First(ctx)
	return player
}
