package main

import (
	"encoding/json"
	"github.com/davecgh/go-spew/spew"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"log"
	"net/http"
	"os"
	"time"
	"token_blockchain/blockchain"
)

type Message struct {
	BPM int
}

func handleGetBlockchain(c *gin.Context) {
	bytes, err := json.MarshalIndent(blockchain.Blockchain, "", "  ")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Data(http.StatusOK, "application/json", bytes)
}

func handleWriteBlock(c *gin.Context) {
	var m Message
	if err := c.ShouldBindJSON(&m); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	//创建新节点
	newBlock, err := blockchain.GenerateBlock(blockchain.Blockchain[len(blockchain.Blockchain)-1], m.BPM)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if blockchain.IsBlockValid(newBlock, blockchain.Blockchain[len(blockchain.Blockchain)-1]) {
		newBlockChain := append(blockchain.Blockchain, newBlock)
		blockchain.ReplaceChain(newBlockChain)
		spew.Dump(newBlock)
	}

	c.JSON(http.StatusCreated, newBlock)
}

func run() error {
	//还有一个debug mode，和一个test mode，我选择debug mode
	gin.SetMode(gin.ReleaseMode)
	router := gin.Default()
	router.GET("/", handleGetBlockchain)
	router.POST("/", handleWriteBlock)
	return router.Run(":" + os.Getenv("ADDR"))
}

func main() {
	//获取.env的配置数据
	err := godotenv.Load()
	if err != nil {
		log.Fatal(err)
	}
	go func() {
		if len(blockchain.Blockchain) == 0 {
			t := time.Now()
			genesisBlock := blockchain.Block{0, t.String(), 0, "", ""}
			//格式化输出
			spew.Dump(genesisBlock)
			blockchain.Blockchain = append(blockchain.Blockchain, genesisBlock)
		}
	}()

	log.Fatal(run())
}
