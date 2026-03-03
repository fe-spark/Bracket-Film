package handler

import (
	"fmt"
	"path/filepath"
	"strconv"

	"server-v2/config"
	"server-v2/internal/service"
	"server-v2/pkg/response"
	"server-v2/pkg/utils"

	"github.com/gin-gonic/gin"
)

type FileHandler struct{}

var FileHd = new(FileHandler)

// SingleUpload 单文件上传, 暂定为图片上传
func (h *FileHandler) SingleUpload(c *gin.Context) {
	v, ok := c.Get(config.AuthUserClaims)
	if !ok {
		response.Failed("上传失败, 当前用户信息异常", c)
		return
	}
	file, err := c.FormFile("file")
	if err != nil {
		response.Failed(err.Error(), c)
		return
	}

	fileName := fmt.Sprintf("%s/%s%s", config.FilmPictureUploadDir, utils.RandomString(8), filepath.Ext(file.Filename))
	err = c.SaveUploadedFile(file, fileName)
	if err != nil {
		response.Failed(err.Error(), c)
		return
	}

	uc := v.(*utils.UserClaims)
	link := service.FileSvc.SingleFileUpload(fileName, int(uc.UserID))
	response.Success(link, "上传成功", c)
}

// MultipleUpload 批量文件上传
func (h *FileHandler) MultipleUpload(c *gin.Context) {
	v, ok := c.Get(config.AuthUserClaims)
	if !ok {
		response.Failed("上传失败, 当前用户信息异常", c)
		return
	}
	form, err := c.MultipartForm()
	if err != nil {
		response.Failed(err.Error(), c)
		return
	}
	files := form.File["files"]
	uc := v.(*utils.UserClaims)

	var fileNames []string
	for _, file := range files {
		fileName := fmt.Sprintf("%s/%s%s", config.FilmPictureUploadDir, utils.RandomString(8), filepath.Ext(file.Filename))
		err = c.SaveUploadedFile(file, fileName)
		if err != nil {
			response.Failed(err.Error(), c)
			return
		}
		fileNames = append(fileNames, service.FileSvc.SingleFileUpload(fileName, int(uc.UserID)))
	}

	response.Success(fileNames, "上传成功", c)
}

// DelFile 删除文件
func (h *FileHandler) DelFile(c *gin.Context) {
	id, err := strconv.ParseUint(c.DefaultQuery("id", ""), 10, 64)
	if err != nil {
		response.Failed("操作失败, 未获取到需删除的文件标识信息", c)
		return
	}
	if e := service.FileSvc.RemoveFileById(uint(id)); e != nil {
		response.Failed(fmt.Sprint("删除失败", e.Error()), c)
		return
	}
	response.SuccessOnlyMsg("文件已删除", c)
}

// PhotoWall 照片墙数据
func (h *FileHandler) PhotoWall(c *gin.Context) {
	current, err := strconv.Atoi(c.DefaultQuery("current", "1"))
	if err != nil {
		response.Failed("图片分页数据获取失败, 分页参数异常", c)
		return
	}
	page := response.Page{PageSize: 39, Current: current}
	pl := service.FileSvc.GetPhotoPage(&page)
	response.Success(gin.H{"list": pl, "page": page}, "图片分页数据获取成功", c)
}
