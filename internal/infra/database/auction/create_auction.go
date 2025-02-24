package auction

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	"fullcycle-auction_go/configuration/logger"
	"fullcycle-auction_go/internal/entity/auction_entity"
	"fullcycle-auction_go/internal/internal_error"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

type AuctionEntityMongo struct {
	Id          string                          `bson:"_id"`
	ProductName string                          `bson:"product_name"`
	Category    string                          `bson:"category"`
	Description string                          `bson:"description"`
	Condition   auction_entity.ProductCondition `bson:"condition"`
	Status      auction_entity.AuctionStatus    `bson:"status"`
	Timestamp   int64                           `bson:"timestamp"`
}
type AuctionRepository struct {
	Collection *mongo.Collection
}

func NewAuctionRepository(ctx context.Context, database *mongo.Database) *AuctionRepository {

	repo := &AuctionRepository{
		Collection: database.Collection("auctions"),
	}

	go repo.StartAuctionExpirationScheduler(ctx)
	return repo
}

func (ar *AuctionRepository) CreateAuction(
	ctx context.Context,
	auctionEntity *auction_entity.Auction) *internal_error.InternalError {
	auctionEntityMongo := &AuctionEntityMongo{
		Id:          auctionEntity.Id,
		ProductName: auctionEntity.ProductName,
		Category:    auctionEntity.Category,
		Description: auctionEntity.Description,
		Condition:   auctionEntity.Condition,
		Status:      auctionEntity.Status,
		Timestamp:   auctionEntity.Timestamp.Unix(),
	}
	_, err := ar.Collection.InsertOne(ctx, auctionEntityMongo)
	if err != nil {
		logger.Error("Error trying to insert auction", err)
		return internal_error.NewInternalServerError("Error trying to insert auction")
	}

	return nil
}

func (ar *AuctionRepository) StartAuctionExpirationScheduler(ctx context.Context) {
	ticker := time.NewTicker(getAuctionExpirationInterval() * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			ar.ExpireAuctions(ctx)
		case <-ctx.Done():
			logger.Info("Auction expiration watcher stopped")
			return
		}
	}
}

func (ar *AuctionRepository) ExpireAuctions(ctx context.Context) {
	now := time.Now().Unix()
	filter := bson.M{
		"status":    auction_entity.Active,
		"timestamp": bson.M{"$lt": now},
	}
	update := bson.M{"$set": bson.M{"status": auction_entity.Completed}}

	result, err := ar.Collection.UpdateMany(ctx, filter, update)
	if err != nil {
		logger.Error("Error updating expired auctions", err)
	} else {
		logger.Info(fmt.Sprintf("Closed %d expired auctions", result.ModifiedCount))
	}
}

func getAuctionExpirationInterval() time.Duration {
	value, err := strconv.Atoi(os.Getenv("AUCTION_INTERVAL"))
	if err != nil {
		return 1
	}

	return time.Duration(value)
}
