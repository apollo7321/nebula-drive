package gui

import (
	"fmt"
	"log"
	"strings"

	"codeberg.org/apollo7321/nebula-drive/lib"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/widget"
)

func NewGui(client lib.StorageClient) error {
	app := app.New()
	mainWindow := app.NewWindow("Nebula Drive")

	var currentBucket string
	var currentPath = []string{}
	fileListData := binding.NewStringList()
	fileList := widget.NewListWithData(fileListData, func() fyne.CanvasObject {
		return widget.NewLabel("template")
	}, func(i binding.DataItem, o fyne.CanvasObject) {
		// workaround to avoid out of bounds errors when updating the file list
		val, err := i.(binding.String).Get()
		if err != nil {
			o.(*widget.Label).SetText("")
			return
		}
		o.(*widget.Label).SetText(val)
	})
	fileList.OnSelected = func(id widget.ListItemID) {
		fileName, err := fileListData.GetValue(id)
		if err != nil {
			log.Printf("failed to get file list data for id %v: %v\n", id, err)
			return
		}
		fileList.UnselectAll()
		if fileName == ".." {
			if len(currentPath) > 0 {
				currentPath = currentPath[:len(currentPath)-1]
			}
			// TODO
		} else if strings.HasSuffix(fileName, "/") {
			var filePath string
			if len(currentPath) == 0 {
				filePath = fileName
			} else {
				filePath = fmt.Sprintf("%s/%s", strings.Join(currentPath, "/"), fileName)
			}
			currentPath = append(currentPath, strings.TrimSuffix(fileName, "/"))
			log.Printf("selected file path: %v\n", filePath)
			files, err := client.List(currentBucket, filePath)
			if err != nil {
				log.Printf("failed to get file list for bucket '%v'\n", currentBucket)
				return
			}
			fileListData.Set(files)
		} else {
			// TODO: save current selected item for toolbar actions
			log.Printf("selected file %v\n", fileName)
		}
	}

	buckets, err := client.Buckets()
	if err != nil {
		return err
	}

	bucketListLabel := widget.NewLabel("AWS S3")
	bucketList := widget.NewList(
		func() int {
			return len(buckets)
		},
		func() fyne.CanvasObject {
			return widget.NewLabel("template")
		},
		func(i widget.ListItemID, o fyne.CanvasObject) {
			o.(*widget.Label).SetText(buckets[i])
		})
	bucketList.OnSelected = func(id widget.ListItemID) {
		currentBucket = buckets[id]
		files, err := client.List(currentBucket, "")
		if err != nil {
			log.Printf("failed to get file list for bucket '%v'\n", currentBucket)
			return
		}
		fileList.UnselectAll()
		fileListData.Set(files)
		fileList.Refresh()
		currentPath = []string{}
	}
	bucketListContanier := container.NewBorder(bucketListLabel, nil, nil, nil, bucketList)

	split := container.NewHSplit(container.NewVScroll(bucketListContanier), fileList)
	split.Offset = 0.2

	mainWindow.SetContent(split)

	mainWindow.Resize(fyne.NewSize(600, 400))
	mainWindow.ShowAndRun()
	return nil
}
