package main

import "github.com/gin-gonic/gin"

func (a *AppState) fileProviderItem(c *gin.Context) {
	a.fp.Item(c)
}

func (a *AppState) fileProviderChildren(c *gin.Context) {
	a.fp.Children(c)
}

func (a *AppState) fileProviderContent(c *gin.Context) {
	a.fp.Content(c)
}

func (a *AppState) fileProviderPutContent(c *gin.Context) {
	a.fp.PutContent(c)
}

func (a *AppState) fileProviderRenameItem(c *gin.Context) {
	a.fp.RenameItem(c)
}

func (a *AppState) fileProviderCreateFolder(c *gin.Context) {
	a.fp.CreateFolder(c)
}

func (a *AppState) fileProviderDeleteItem(c *gin.Context) {
	a.fp.DeleteItem(c)
}
