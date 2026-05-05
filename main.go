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

//get block chain
func handleGetBlockchain(c *gin.Context) {
	//marshal转json
	bytes, err := json.MarshalIndent(blockchain.Blockchain, "", "  ")
	if err != nil {
		//都是以Status开头，返回的json数据可以直接用gin.H
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	//PS：C.Data返回需要手动设置ContentType
	c.Data(http.StatusOK, "application/json", bytes)
}

//set block chain
func handleWriteBlock(c *gin.Context) {
	var m Message
	//将请求体解析并自动绑定到结构体上
	if err := c.ShouldBindJSON(&m); err != nil {
		//常用的也就这三个，OK、BadRequest、InternalServerError
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
