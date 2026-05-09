package utils

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"log"
	"os"
)

type RSACrypto struct {
	privateKey *rsa.PrivateKey
	publicKey  *rsa.PublicKey
}

func NewRSACrypto() (*RSACrypto, error) {
	privateKeyPEM := getPrivateKey()
	publicKeyPEM := getPublicKey()

	if privateKeyPEM == "" {
		return nil, fmt.Errorf("未找到RSA私钥配置")
	}

	privateKeyBlock, _ := pem.Decode([]byte(privateKeyPEM))
	if privateKeyBlock == nil {
		return nil, fmt.Errorf("私钥PEM格式错误")
	}

	privateKey, err := x509.ParsePKCS1PrivateKey(privateKeyBlock.Bytes)
	if err != nil {
		return nil, fmt.Errorf("解析私钥失败: %v", err)
	}

	var publicKey *rsa.PublicKey
	if publicKeyPEM != "" {
		publicKeyBlock, _ := pem.Decode([]byte(publicKeyPEM))
		if publicKeyBlock != nil {
			pub, err := x509.ParsePKIXPublicKey(publicKeyBlock.Bytes)
			if err != nil {
				pubKey, err := x509.ParsePKCS1PublicKey(publicKeyBlock.Bytes)
				if err == nil {
					publicKey = pubKey
				}
			} else {
				publicKey = pub.(*rsa.PublicKey)
			}
		}
	}

	if publicKey == nil {
		publicKey = &privateKey.PublicKey
	}

	if publicKey == nil {
		return nil, fmt.Errorf("无法获取公钥")
	}

	return &RSACrypto{
		privateKey: privateKey,
		publicKey:  publicKey,
	}, nil
}

func (r *RSACrypto) Decrypt(encryptedBase64 string) (string, error) {
	if r.privateKey == nil {
		return "", fmt.Errorf("私钥未初始化")
	}

	encryptedData, err := base64.StdEncoding.DecodeString(encryptedBase64)
	if err != nil {
		return "", fmt.Errorf("Base64解码失败: %v", err)
	}

	decrypted, err := rsa.DecryptOAEP(sha1.New(), nil, r.privateKey, encryptedData, nil)
	if err != nil {
		return "", fmt.Errorf("RSA解密失败: %v", err)
	}

	return string(decrypted), nil
}

func (r *RSACrypto) Encrypt(data string) (string, error) {
	if r.publicKey == nil {
		return "", fmt.Errorf("公钥未初始化")
	}

	encrypted, err := rsa.EncryptOAEP(sha1.New(), rand.Reader, r.publicKey, []byte(data), nil)
	if err != nil {
		return "", fmt.Errorf("RSA加密失败: %v", err)
	}

	return base64.StdEncoding.EncodeToString(encrypted), nil
}

func getPrivateKey() string {
	if key := os.Getenv("RSA_PRIVATE_KEY"); key != "" {
		return key
	}

	if data, err := os.ReadFile("security/rsa_private_key.pem"); err == nil {
		return string(data)
	}

	if data, err := os.ReadFile("../security/rsa_private_key.pem"); err == nil {
		return string(data)
	}

	log.Println("警告: 未找到RSA私钥，请配置RSA_PRIVATE_KEY环境变量或security/rsa_private_key.pem文件")
	return ""
}

func getPublicKey() string {
	if key := os.Getenv("RSA_PUBLIC_KEY"); key != "" {
		return key
	}

	if data, err := os.ReadFile("security/rsa_public_key.pem"); err == nil {
		return string(data)
	}

	if data, err := os.ReadFile("../security/rsa_public_key.pem"); err == nil {
		return string(data)
	}

	return ""
}

var globalRSACrypto *RSACrypto

func InitRSACrypto() error {
	if globalRSACrypto != nil {
		return nil
	}

	var err error
	globalRSACrypto, err = NewRSACrypto()
	if err != nil {
		log.Printf("❌ RSA加密解密器初始化失败: %v", err)
		return err
	}

	log.Printf("✅ RSA加密解密器初始化成功")
	return nil
}

func DecryptWithRSA(encryptedBase64 string) (string, error) {
	if globalRSACrypto == nil {
		return "", fmt.Errorf("RSA加密解密器未初始化")
	}
	return globalRSACrypto.Decrypt(encryptedBase64)
}

func EncryptWithRSA(data string) (string, error) {
	if globalRSACrypto == nil {
		return "", fmt.Errorf("RSA加密解密器未初始化")
	}
	return globalRSACrypto.Encrypt(data)
}