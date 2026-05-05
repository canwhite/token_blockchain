package blockchain

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

//type xxx struct创建class
type Block struct {
	Index     int
	Timestamp string
	BPM       int   //我们自己的数据
	Hash      string
	PrevHash  string
}

var Blockchain []Block

//输入拼接: Index + Timestamp + BPM + PrevHash
func calculateHash(block Block) string {
	//注意BPM是我们的信息
	record := fmt.Sprint(block.Index) + block.Timestamp + fmt.Sprint(block.BPM) + block.PrevHash
	//创建哈希器
	h := sha256.New()
	h.Write([]byte(record))
	//输出字节，nil 表示"不需要追加到任何现有切片，直接给我哈希结果就行"。
	//我可以不传nil，传进去一个切面，然后就会在切片后边拼接了是吧
	hashed := h.Sum(nil)
	//转化成字符串
	return hex.EncodeToString(hashed)
}

//==>返回值是个元组
func GenerateBlock(oldBlock Block, BPM int) (Block, error) {
	//直接定义，而不是短变量声明
	var newBlock Block
	//Now，返回的是一个结构体，fmt.Println(now.Format("2006-01-02 15:04:05"))中的2006-01-02 15:04:05，是语法规则
	t := time.Now()
	newBlock.Index = oldBlock.Index + 1
	newBlock.Timestamp = t.String()
	newBlock.BPM = BPM
	newBlock.PrevHash = oldBlock.Hash
	newBlock.Hash = calculateHash(newBlock)
	return newBlock, nil
}

//判断block是否valid
func IsBlockValid(newBlock Block, oldBlock Block) bool {
	//断链了
	if oldBlock.Index+1 != newBlock.Index {
		return false
	}
	//前hash不对
	if oldBlock.Hash != newBlock.PrevHash {
		return false
	}
	//现hash不对
	if calculateHash(newBlock) != newBlock.Hash {
		return false
	}
	return true
}
//根据长度选择链
func ReplaceChain(newBlocks []Block) {
	if len(newBlocks) > len(Blockchain) {
		Blockchain = newBlocks
	}
}
