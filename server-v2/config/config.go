package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

/*
 Global Configuration Variables
*/

var (
	// ListenerPort web服务监听的端口
	ListenerPort = ""
	// MysqlDsn mysql服务配置信息
	MysqlDsn = ""

	// Redis连接信息
	RedisAddr     = ""
	RedisPassword = ""
	RedisDBNo     = 0
)

const (
	// MAXGoroutine max goroutine, 执行spider中对协程的数量限制
	MAXGoroutine = 10

	FilmPictureUploadDir = "./static/upload/gallery"
	FilmPictureUrlPath   = "/upload/pic/poster/"
	FilmPictureAccess    = "/api/upload/pic/poster/"
)

// -------------------------redis key-----------------------------------
const (
	// CategoryTreeKey 分类树 key
	CategoryTreeKey = "CategoryTree"
	// ConfigCacheTTL 管理员写入控制的配置类 key 有效期 (以长 TTL 最大化命中率)
	ConfigCacheTTL = time.Hour * 24

	// VirtualPictureKey 待同步图片临时存储 key
	VirtualPictureKey = "VirtualPicture"
	// MaxScanCount redis Scan 操作每次扫描的数据量, 每次最多扫描300条数据
	MaxScanCount = 300
)

const (
	AuthUserClaims = "UserClaims"
)

// -------------------------manage 管理后台相关key----------------------------------
const (
	// SiteConfigBasic 网站参数配置
	SiteConfigBasic = "SystemConfig:SiteConfig:Basic"
	// BannersKey 轮播组件key
	BannersKey = "SystemConfig:Banners"

	// DefaultUpdateSpec 每20分钟执行一次
	DefaultUpdateSpec = "0 */20 * * * ?"
	// EveryWeekSpec 每周日凌晨4点更新一次
	EveryWeekSpec = "0 0 4 * * 0"
	// DefaultUpdateTime 每次采集最近 3 小时内更新的影片
	DefaultUpdateTime = 3
	// DefaultSpiderInterval 默认采集间隔 (ms)，当站点未配置时使用
	DefaultSpiderInterval = 500
)

// -------------------------Database Connection Params-----------------------------------
const (
	// SearchTableName 存放检索信息的数据表名
	SearchTableName  = "search"
	UserTableName    = "users"
	UserIdInitialVal = 10000
)

// -------------------------Provide Config-----------------------------------
const (
	PlayForm      = "bkm3u8"
	PlayFormCloud = "bracket"
	PlayFormAll   = "bracket$$$bkm3u8"
	RssVersion    = "5.1"
)

// -------------------------Security Config-----------------------------------
const PrivateKey = `-----BEGIN RSA PRIVATE KEY-----
MIIBPAIBAAJBANNnshoUaT2gFNrihmFdmC1cBCs1XLFc5Fn3MfNOR3aOGDO0ohXl
bku6Ir/qITN/yeH5pY34WEcETet3YhESpE8CAwEAAQJBAI7Ekdfg/u26RTtJDd2F
WrcPVFVl1TKGfERxl08sB0D9HLvUSBfAEg/UpfWSQ57aSJ9b0gVKmDhgF8FymuUV
v2kCIQDzXXSZ/oeKmqObwad0Fa82IFof3LeZdpbrjyz3w45JDQIhAN5hdmuW+y2w
UgSy0o4zGFsEG/RBZsvVnSSfkdR47dPLAiEA2XbPNLQu5fnc7NeVDLQ7xsAOCJ6w
KR/BKGjeI9/JCxkCIQCjMkU0ec2FXxMhzZXFs2uZR6+4FdL5nZ9ABDaCBekK9wIg
XEfd11qabi9jPrbsOVNZCTk51B7Ug0ZwGyn0BA8Jlo0=
-----END RSA PRIVATE KEY-----
`

const PublicKey = `-----BEGIN RSA PUBLIC KEY-----
MEgCQQDTZ7IaFGk9oBTa4oZhXZgtXAQrNVyxXORZ9zHzTkd2jhgztKIV5W5LuiK/
6iEzf8nh+aWN+FhHBE3rd2IREqRPAgMBAAE=
-----END RSA PUBLIC KEY-----
`

const (
	Issuer           = "Bracket"
	AuthTokenExpires = 10 * 24 // 单位 h
	UserTokenKey     = "User:Token:%d"
)

// init func for loading from env
func init() {
	InitConfig()
}

func InitConfig() {
	// 加载监听端口
	if port := os.Getenv("PORT"); port != "" {
		ListenerPort = port
	} else if lPort := os.Getenv("LISTENER_PORT"); lPort != "" {
		ListenerPort = lPort
	}
	if ListenerPort == "" {
		panic("环境变量缺失: PORT 或 LISTENER_PORT")
	}
	fmt.Printf("[Config] 加载端口: %s\n", ListenerPort)

	// 加载 MySQL 配置
	mHost := os.Getenv("MYSQL_HOST")
	mPort := os.Getenv("MYSQL_PORT")
	mUser := os.Getenv("MYSQL_USER")
	mPass := os.Getenv("MYSQL_PASSWORD")
	mDB := os.Getenv("MYSQL_DBNAME")

	if mHost == "" || mPort == "" || mUser == "" || mDB == "" {
		panic(fmt.Sprintf("环境变量缺失: MYSQL_HOST=%s, MYSQL_PORT=%s, MYSQL_USER=%s, MYSQL_DBNAME=%s",
			mHost, mPort, mUser, mDB))
	}

	MysqlDsn = fmt.Sprintf("%s:%s@(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		mUser, mPass, mHost, mPort, mDB)
	fmt.Printf("[Config] 加载 MySQL DSN: %s:%s@(%s:%s)/%s\n", mUser, "******", mHost, mPort, mDB)

	// 加载 Redis 配置
	rHost := os.Getenv("REDIS_HOST")
	rPort := os.Getenv("REDIS_PORT")
	rPass := os.Getenv("REDIS_PASSWORD")
	rDB := os.Getenv("REDIS_DB")

	if rHost == "" || rPort == "" {
		panic(fmt.Sprintf("环境变量缺失: REDIS_HOST=%s, REDIS_PORT=%s", rHost, rPort))
	}

	RedisAddr = fmt.Sprintf("%s:%s", rHost, rPort)
	if rPass != "" {
		RedisPassword = rPass
	}
	if rDB != "" {
		if dbNo, err := strconv.Atoi(rDB); err == nil {
			RedisDBNo = dbNo
		}
	}
	fmt.Printf("[Config] 加载 Redis 地址: %s, DB: %d\n", RedisAddr, RedisDBNo)
}
