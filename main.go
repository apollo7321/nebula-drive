package main

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"log"
	"os"
	"strings"
	"time"

	"gioui.org/app"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// AppState contains the entire application state
type AppState struct {
	window *app.Window
	theme  *material.Theme

	// List widgets
	bucketList widget.List
	objectList widget.List

	// Clickable states
	bucketClickables []*widget.Clickable
	objectClickables []*widget.Clickable
	deleteButton     widget.Clickable
	confirmButton    widget.Clickable
	cancelButton     widget.Clickable

	// Application data and state
	buckets             []string
	objects             []string
	selectedBucket      string
	currentPrefix       string
	selectedObjectIndex int
	showConfirmDelete   bool
}

// --- S3 Backend Logic ---
func loadBuckets() ([]string, error) {
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return nil, err
	}
	s3Client := s3.NewFromConfig(cfg)
	result, err := s3Client.ListBuckets(context.TODO(), &s3.ListBucketsInput{})
	if err != nil {
		return nil, err
	}
	var buckets []string
	for _, bucket := range result.Buckets {
		buckets = append(buckets, *bucket.Name)
	}
	return buckets, nil
}

func loadObjects(bucketName string, prefix string) ([]string, error) {
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return nil, err
	}
	s3Client := s3.NewFromConfig(cfg)
	input := &s3.ListObjectsV2Input{
		Bucket: &bucketName, Delimiter: aws.String("/"), Prefix: aws.String(prefix),
	}
	result, err := s3Client.ListObjectsV2(context.TODO(), input)
	if err != nil {
		return nil, err
	}
	var items []string
	// ".." wird jetzt in loadObjectsAsync hinzugefügt
	for _, p := range result.CommonPrefixes {
		items = append(items, strings.TrimPrefix(*p.Prefix, prefix))
	}
	for _, object := range result.Contents {
		if *object.Key != prefix {
			items = append(items, strings.TrimPrefix(*object.Key, prefix))
		}
	}
	return items, nil
}

func deleteS3Objects(keys []string, bucketName string) error {
	if len(keys) == 0 {
		return nil
	}
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return err
	}
	s3Client := s3.NewFromConfig(cfg)
	var objectIds []types.ObjectIdentifier
	for _, key := range keys {
		objectIds = append(objectIds, types.ObjectIdentifier{Key: aws.String(key)})
	}
	_, err = s3Client.DeleteObjects(context.TODO(), &s3.DeleteObjectsInput{
		Bucket: aws.String(bucketName),
		Delete: &types.Delete{Objects: objectIds},
	})
	return err
}

func listAllKeysForPrefix(bucketName, prefix string) ([]string, error) {
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return nil, err
	}
	s3Client := s3.NewFromConfig(cfg)
	var keys []string
	paginator := s3.NewListObjectsV2Paginator(s3Client, &s3.ListObjectsV2Input{
		Bucket: aws.String(bucketName), Prefix: aws.String(prefix),
	})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(context.TODO())
		if err != nil {
			return nil, err
		}
		for _, obj := range page.Contents {
			keys = append(keys, *obj.Key)
		}
	}
	return keys, nil
}

// --- Main Application ---
func main() {
	go func() {
		w := new(app.Window)
		w.Option(app.Title("Nebula Drive (Gio)"), app.Decorated(true))
		if err := run(w); err != nil {
			log.Fatal(err)
		}
		os.Exit(0)
	}()
	app.Main()
}

func run(window *app.Window) error {
	var lastTappedID int = -1
	var lastTapped time.Time
	const doubleTapDelay = 500 * time.Millisecond

	state := &AppState{
		window:              window,
		theme:               material.NewTheme(),
		selectedObjectIndex: -1,
	}
	state.bucketList.Axis = layout.Vertical
	state.objectList.Axis = layout.Vertical

	go func() {
		buckets, err := loadBuckets()
		if err != nil {
			state.buckets = []string{fmt.Sprintf("Fehler: %v", err)}
		} else {
			state.buckets = buckets
		}
		state.bucketClickables = make([]*widget.Clickable, len(state.buckets))
		for i := range state.bucketClickables {
			state.bucketClickables[i] = new(widget.Clickable)
		}
		state.window.Invalidate()
	}()

	var ops op.Ops
	for {
		switch e := window.Event().(type) {
		case app.DestroyEvent:
			return e.Err
		case app.FrameEvent:
			gtx := app.NewContext(&ops, e)

			// Handle all clicks before laying out the frame
			for i, clickable := range state.bucketClickables {
				if clickable.Clicked(gtx) {
					state.selectedBucket = state.buckets[i]
					state.currentPrefix = ""
					state.selectedObjectIndex = -1
					state.loadObjectsAsync()
				}
			}

			for i, clickable := range state.objectClickables {
				if clickable.Clicked(gtx) {
					if i == lastTappedID && time.Since(lastTapped) < doubleTapDelay {
						// Double-click
						item := state.objects[i]
						if item == ".." {
							if state.currentPrefix != "" {
								parts := strings.Split(strings.TrimSuffix(state.currentPrefix, "/"), "/")
								newPrefix := strings.Join(parts[:len(parts)-1], "/")
								if newPrefix != "" {
									newPrefix += "/"
								}
								state.currentPrefix = newPrefix
								state.selectedObjectIndex = -1
								state.loadObjectsAsync()
							}
						} else if strings.HasSuffix(item, "/") {
							state.currentPrefix += item
							state.selectedObjectIndex = -1
							state.loadObjectsAsync()
						}
						lastTappedID = -1 // Reset after action
					} else {
						// Single-click
						state.selectedObjectIndex = i
						log.Printf("Item ausgewählt: %s", state.objects[i])
						lastTappedID = i
						lastTapped = time.Now()
					}
				}
			}

			if state.deleteButton.Clicked(gtx) {
				if state.selectedObjectIndex != -1 {
					state.showConfirmDelete = true
				}
			}
			if state.cancelButton.Clicked(gtx) {
				state.showConfirmDelete = false
			}
			if state.confirmButton.Clicked(gtx) {
				state.showConfirmDelete = false
				state.deleteSelectedObjectAsync()
			}

			state.Layout(gtx)
			e.Frame(gtx.Ops)
		}
	}
}

// --- Layout & Logic ---
func (s *AppState) Layout(gtx layout.Context) {
	border := widget.Border{
		Color: color.NRGBA{A: 255},
		Width: unit.Dp(1),
	}

	border.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.UniformInset(unit.Dp(8)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			separator := layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				dims := layout.Dimensions{Size: image.Point{X: gtx.Dp(1), Y: gtx.Constraints.Max.Y}}
				rect := clip.Rect{Max: dims.Size}
				paint.FillShape(gtx.Ops, color.NRGBA{A: 50}, rect.Op())
				return dims
			})

			layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
				layout.Flexed(0.3, s.layoutBucketList),
				separator,
				layout.Flexed(0.7, s.layoutObjectView),
			)
			return layout.Dimensions{Size: gtx.Constraints.Max}
		})
	})

	if s.showConfirmDelete {
		s.layoutDeleteConfirm(gtx)
	}
}

func (s *AppState) layoutBucketList(gtx layout.Context) layout.Dimensions {
	return layout.UniformInset(unit.Dp(8)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return material.List(s.theme, &s.bucketList).Layout(gtx, len(s.buckets), func(gtx layout.Context, i int) layout.Dimensions {
			return material.Clickable(gtx, s.bucketClickables[i], func(gtx layout.Context) layout.Dimensions {
				return layout.UniformInset(unit.Dp(8)).Layout(gtx, material.Body1(s.theme, s.buckets[i]).Layout)
			})
		})
	})
}

func (s *AppState) layoutObjectView(gtx layout.Context) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.UniformInset(unit.Dp(8)).Layout(gtx,
				func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{
						Axis:      layout.Horizontal,
						Spacing:   layout.SpaceBetween,
						Alignment: layout.Middle,
					}.Layout(gtx,
						layout.Rigid(material.Body1(s.theme, s.selectedBucket+"/"+s.currentPrefix).Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							btn := material.Button(s.theme, &s.deleteButton, "Löschen")

							// KORRIGIERT: Prüfe zusätzlich, ob ".." ausgewählt ist.
							isSelected := s.selectedObjectIndex != -1
							isUpDirectorySelected := isSelected && s.objects[s.selectedObjectIndex] == ".."

							if !isSelected || isUpDirectorySelected {
								gtx = gtx.Disabled()
							}
							return btn.Layout(gtx)
						}),
					)
				},
			)
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return material.List(s.theme, &s.objectList).Layout(gtx, len(s.objects), func(gtx layout.Context, i int) layout.Dimensions {
				itemToLayout := s.objects[i]
				isRootUpDisabled := itemToLayout == ".." && s.currentPrefix == ""

				itemGtx := gtx
				if isRootUpDisabled {
					itemGtx = gtx.Disabled()
				}

				return material.Clickable(itemGtx, s.objectClickables[i], func(gtx layout.Context) layout.Dimensions {
					macro := op.Record(gtx.Ops)
					dims := layout.UniformInset(unit.Dp(8)).Layout(gtx,
						material.Body1(s.theme, itemToLayout).Layout,
					)
					call := macro.Stop()

					if s.selectedObjectIndex == i {
						paint.FillShape(gtx.Ops, s.theme.Palette.ContrastBg, clip.Rect{Max: dims.Size}.Op())
					}
					call.Add(gtx.Ops)
					return dims
				})
			})
		}),
	)
}

func (s *AppState) layoutDeleteConfirm(gtx layout.Context) {
	paint.Fill(gtx.Ops, color.NRGBA{A: 180})
	layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		border := widget.Border{
			Color:        color.NRGBA{A: 255},
			CornerRadius: unit.Dp(8),
			Width:        unit.Dp(1),
		}

		return border.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			macro := op.Record(gtx.Ops)
			dims := layout.UniformInset(unit.Dp(16)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(material.H6(s.theme, "Löschen bestätigen").Layout),
					layout.Rigid(layout.Spacer{Height: unit.Dp(16)}.Layout),
					layout.Rigid(material.Body1(s.theme, fmt.Sprintf("Möchten Sie '%s' wirklich löschen?", s.objects[s.selectedObjectIndex])).Layout),
					layout.Rigid(layout.Spacer{Height: unit.Dp(16)}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{}.Layout(gtx,
							layout.Flexed(1, material.Button(s.theme, &s.cancelButton, "Abbrechen").Layout),
							layout.Flexed(1, material.Button(s.theme, &s.confirmButton, "Löschen").Layout),
						)
					}),
				)
			})
			call := macro.Stop()

			radius := gtx.Dp(unit.Dp(8))
			backgroundClip := clip.RRect{
				Rect: image.Rectangle{Max: image.Point{X: dims.Size.X, Y: dims.Size.Y}},
				SE:   radius, SW: radius, NW: radius, NE: radius,
			}.Op(gtx.Ops)
			paint.FillShape(gtx.Ops, s.theme.Palette.Bg, backgroundClip)

			call.Add(gtx.Ops)
			return dims
		})
	})
}

// --- Asynchronous Helper Functions ---
func (s *AppState) loadObjectsAsync() {
	go func() {
		objects, err := loadObjects(s.selectedBucket, s.currentPrefix)
		if err != nil {
			s.objects = []string{"..", fmt.Sprintf("Fehler: %v", err)}
		} else {
			s.objects = append([]string{".."}, objects...)
		}
		s.objectClickables = make([]*widget.Clickable, len(s.objects))
		for i := range s.objectClickables {
			s.objectClickables[i] = new(widget.Clickable)
		}
		s.window.Invalidate()
	}()
}

func (s *AppState) deleteSelectedObjectAsync() {
	if s.selectedObjectIndex < 0 || s.selectedObjectIndex >= len(s.objects) {
		return
	}
	itemToDelete := s.objects[s.selectedObjectIndex]
	isFolder := strings.HasSuffix(itemToDelete, "/")
	fullPath := s.currentPrefix + itemToDelete
	log.Printf("Lösche '%s'...", fullPath)

	go func() {
		var keysToDelete []string
		var err error
		if isFolder {
			keysToDelete, err = listAllKeysForPrefix(s.selectedBucket, fullPath)
		} else {
			keysToDelete = []string{fullPath}
		}

		if err != nil {
			log.Printf("Fehler beim Auflisten der Objekte: %v", err)
			return
		}

		err = deleteS3Objects(keysToDelete, s.selectedBucket)
		if err != nil {
			log.Printf("Fehler beim Löschen: %v", err)
			return
		}

		log.Printf("Erfolgreich gelöscht.")
		s.selectedObjectIndex = -1
		s.loadObjectsAsync()
	}()
}
