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

	// IsDevMode 是否处于开发模式 (开发模式下每次启动会清空数据库和 Redis)
	IsDevMode = false

	// MySQL 原始分项配置 (用于建库与重建等底层操作)
	MysqlHost   = ""
	MysqlPort   = ""
	MysqlUser   = ""
	MysqlPass   = ""
	MysqlDBName = ""
)

const (
	// MAXGoroutine max goroutine, 执行spider中对协程的数量限制
	MAXGoroutine = 10

	FilmPictureUploadDir = "./static/upload/gallery"
	FilmPictureAccess    = "/api/upload/pic/poster/"
)

// -------------------------redis key-----------------------------------
const (
	// CategoryTreeKey 分类树 key
	CategoryTreeKey = "Category:Tree"
	// ActiveCategoryTreeKey 活跃分类树缓存 key
	ActiveCategoryTreeKey = "Category:ActiveTree"
	// ConfigCacheTTL 管理员写入控制的配置类 key 有效期 (以长 TTL 最大化命中率)
	ConfigCacheTTL = time.Hour * 24

	// SearchTags 搜索分类标签缓存 key (前缀)
	SearchTags = "Search:Tags"
	// TVBoxConfigCacheKey TVBox 分类及筛选配置缓存 key
	TVBoxConfigCacheKey = "TVBox:Config"
	// IndexPageCacheKey 首页数据缓存 key
	IndexPageCacheKey = "Index:Page"
	// TVBoxList TVBox 列表页缓存前缀
	TVBoxList = "TVBox:List"

	// VirtualPictureKey 待同步图片临时存储 key
	VirtualPictureKey = "Gallery:VirtualPicture"
	// MaxScanCount redis Scan 操作每次扫描的数据量, 每次最多扫描300条数据
	MaxScanCount = 300
)

const (
	AuthUserClaims = "UserClaims"
)

// -------------------------manage 管理后台相关key----------------------------------
const (
	// SiteConfigBasic 网站参数配置
	SiteConfigBasic = "Config:Site:Basic"
	// BannersKey 轮播组件key
	BannersKey = "Config:Banners"

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
const PrivateKey = `-----BEGIN PRIVATE KEY-----
MIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQDCyH31rRURSXGV
vAn4yYFmanQfnt7thJO3NVsDPSo3p/BJTSj5tAdVUi5Uu54JadU6k7qz7aI1Ou7H
BVsxX0z0EXqCOFDHKZMctEoZwVXlUTxDSl7Kf97AlHdh2ZJT951ifGHLF8TTaEtK
ulBAjX5Hb7BCH2kChhVuR3PLiQZNLUBnhPIyBkyuBOR1/rI1MZbMJOBoPuU+tqwM
g+1xPoT2WLnW11tOblmEO2b6/7oUaoB+EffJcdE4igPQAIWhXdRKMFtavFRPrFCT
ASVa/hP0V8y5XVi7DPj5nbdmH1YOJuxchhUnmVLPn4AMnDKcmlU7fyxRBb/tj1+r
3hFaJSmHAgMBAAECggEAC74okdoYbg1ecqd+dmg3i+QZEhry16Dpgt8NmJlkZSyT
uOeU89DdrFAjCOqysWCXAUwMsnI+GDVwVcFF6SkUq5YuK5GXlRo1i0J3QSw9sHCA
UJI4Or8Qv81zkQub3cIM0/YpsyPAsvoTp/Kpieq91TKvjpz0KLnKqvZVzcx5+8cG
nX5bLeD8+5JCvkJBITqDAyrZvkvBZW3tJ7h1D6tbb8iz6VZig/jie9ZdGZAEPZ7p
PhCLsA6mYQDgX0tD98/SJODOxqjqlkPx9T/t9DnQ4qXJ/62TiCU+E0vvgnC7pJhA
drTn7kGng3usRZpiE7pLnCfIlcOxcZ7y53lCgxH5qQKBgQDw2FHtTxBrDvded/mv
MVbchTmNSXZR3GXxRweE0n29F+9maA/1DHHiwXIKlFDOsfIe6UbAIAxMkULrQJEC
Bxk7/V11KdVBsU3ACBITdK0+S1qVlGeRPZ3X4BIxybpd2/zyt4qE2gZ2cZCOsckM
Sjya+4lvqZf3HsNMqO6+HY+KwwKBgQDPCi38g1BoOfV27SxhlSUo9yrDdovy4MWU
XyMSgEXcNHLONte8q2EwSIfbylQXZzC6ajzySxmNOomMyghYcepGfDDv8dA/1TcX
PHnYSblni6ieItlzZ99WdHMsCqhZfrEyJbQvO0ORdPu3BcXjQMhrwWRr4rJzxzDb
+WZJj55R7QKBgQCzUp1Nb/zteWs9b178zmO6NYewZu4t7UgJ6bTzdDYiwNuDCCA5
eFajWx0qO1wfSebYlSAUlMgTimSk/KH7PIXRYMhhIBCkpPsa6+dpjQogw8JidOjX
/2SzAycI4wZcNBuWLIp6eEsvjUbwt/bVq8CMNJUUCtYXLVSEk5OPAjuKOQKBgFbx
Pmh4uE5ccHD1nhqIaCdwy/tzD8f5jd8FqJO/XBbhy4g/TY9EJLcC7lJk/7UoNzVB
IcDZuqws9dAykxiZFblts5s/X6U+ozjVw5EJPJt38WIe3lPxPb9vfWH0Q8f5RO37
GVRwPaqaho3QFc6dyMw/VS1c8HVgI2tsqwCfF+vtAoGAWH8xW2JDSpi6TyFRp2Kt
jf6Lx5FH3bHSIoksmdbcnhll7B9J4t6yVPkpCoUVsE0ug7TDTKor+JsHQT/2lIGK
D3YTriSna7YkDbkt4RVlA4ju5UrSiqQyuKL5GQrdWVG629appHB9+5FcbSupywZJ
b9asaRoJpW1umG1RMrF5XV4=
-----END PRIVATE KEY-----
`

const PublicKey = `-----BEGIN PUBLIC KEY-----
MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAwsh99a0VEUlxlbwJ+MmB
Zmp0H57e7YSTtzVbAz0qN6fwSU0o+bQHVVIuVLueCWnVOpO6s+2iNTruxwVbMV9M
9BF6gjhQxymTHLRKGcFV5VE8Q0peyn/ewJR3YdmSU/edYnxhyxfE02hLSrpQQI1+
R2+wQh9pAoYVbkdzy4kGTS1AZ4TyMgZMrgTkdf6yNTGWzCTgaD7lPrasDIPtcT6E
9li51tdbTm5ZhDtm+v+6FGqAfhH3yXHROIoD0ACFoV3USjBbWrxUT6xQkwElWv4T
9FfMuV1Yuwz4+Z23Zh9WDibsXIYVJ5lSz5+ADJwynJpVO38sUQW/7Y9fq94RWiUp
hwIDAQAB
-----END PUBLIC KEY-----
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

	// 检测开发模式 (ENV=dev 或 IS_DEV_MODE=true)
	env := os.Getenv("ENV")
	devFlag := os.Getenv("IS_DEV_MODE")
	if env == "dev" || devFlag == "true" {
		IsDevMode = true
		fmt.Println("[Config] 检测到开发模式：已开启数据库与 Redis 自动清空机制")
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

	MysqlHost = mHost
	MysqlPort = mPort
	MysqlUser = mUser
	MysqlPass = mPass
	MysqlDBName = mDB

	MysqlDsn = fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local&timeout=10s&readTimeout=30s&interpolateParams=true",
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

// GetRootMysqlDsn 获取不带数据库名的 DSN，用于 CREATE DATABASE 等管理操作
func GetRootMysqlDsn() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%s)/?charset=utf8mb4&parseTime=True&loc=Local",
		MysqlUser, MysqlPass, MysqlHost, MysqlPort)
}
