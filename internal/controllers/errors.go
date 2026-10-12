package controllers

import "errors"

var (
	errUnnamedDownload = errors.New("the download's file name cannot go in a Content-Disposition header")
)
