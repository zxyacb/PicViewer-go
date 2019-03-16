package main

import (
	"fmt"
	"github.com/go-gl/gl/v3.2-core/gl"
	"github.com/inkyblackness/imgui-go"
	"image"
	"image/draw"
	"image/jpeg"
	"image/png"
	"os"
	"picViewer/platforms"
	"strings"
	"sync"
	"time"
)

// Platform covers mouse/keyboard/gamepad inputs, cursor shape, timing, windowing.
type Platform interface {
	// ShouldStop is regularly called as the abort condition for the program loop.
	ShouldStop() bool
	// ProcessEvents is called once per render loop to dispatch any pending events.
	ProcessEvents()
	// DisplaySize returns the dimension of the display.
	DisplaySize() [2]float32
	// FramebufferSize returns the dimension of the framebuffer.
	FramebufferSize() [2]float32
	// NewFrame marks the begin of a render pass. It must update the imgui IO state according to user input (mouse, keyboard, ...)
	NewFrame()
	// PostRender marks the completion of one render pass. Typically this causes the display buffer to be swapped.
	PostRender()
}

// Renderer covers rendering imgui draw data.
type Renderer interface {
	// PreRender causes the display buffer to be prepared for new output.
	PreRender(clearColor [4]float32)
	// Render draws the provided imgui draw data.
	Render(displaySize [2]float32, framebufferSize [2]float32, drawData imgui.DrawData)
}

type ImageInfo struct {
	filePath string
	handle   uint32
	width    int32
	height   int32
	rgba     *image.RGBA
}

var (
	mutex             sync.Mutex
	dirPath           string
	images            []ImageInfo
	stoploading             = false
	load                    = true
	imageColumns      int32 = 3
	enlargeImageIndex       = -1
)
// Run implements the main program loop of the demo. It returns when the platform signals to stop.
// This demo application shows some basic features of ImGui, as well as exposing the standard demo window.
func Run(p *platforms.GLFW, r Renderer) {
	showDemoWindow := false
	clearColor := [4]float32{0, 0, 0, 1.0}
	//f := float32(0.0)
	//counter := 0
	showAnotherWindow := false

	p.DropCallback = func(f []string) {
		if len(f) > 0 {
			drop, _ := os.Open(f[0])
			fs, _ := drop.Readdir(-1)
			if len(fs) > 0 {
				dirPath = drop.Name()
				// waiting for last loadimage routine stop
				stoploading = true
				mutex.Lock()
				stoploading = false
				for _, v := range images {
					if v.width > 0 {
						gl.DeleteTextures(1, &v.handle)
						v.width = 0
					}
				}
				mutex.Unlock()
				images = make([]ImageInfo, len(fs))
				for i := 0; i < len(fs); i++ {
					images[i].filePath = dirPath + "\\" + fs[i].Name()
				}
				load = false
			}
		}
	}

	for !p.ShouldStop() {
		p.ProcessEvents()

		// Signal start of a new frame
		p.NewFrame()
		imgui.NewFrame()

		// 1. Show the big demo window (Most of the sample code is in ImGui::ShowDemoWindow()!
		// You can browse its code to learn more about Dear ImGui!).
		if showDemoWindow {
			imgui.ShowDemoWindow(&showDemoWindow)
		}
		imgui.SetNextWindowPos(imgui.Vec2{0, 0})
		size := p.DisplaySize()
		imgui.SetNextWindowSize(imgui.Vec2{size[0], size[1]})

		// 2. Show a simple window that we create ourselves. We use a Begin/End pair to created a named window.
		{
			imgui.CurrentStyle().SetColor(imgui.StyleColorWindowBg, imgui.Vec4{0.875, 0.875, 0.875, 1})
			imgui.CurrentStyle().SetColor(imgui.StyleColorText, imgui.Vec4{0, 0, 0, 1})
			imgui.BeginV("Hello, world!", nil, imgui.WindowFlagsNoTitleBar|imgui.WindowFlagsNoResize) // Create a window called "Hello, world!" and append into it.
			//imgui.Checkbox("Demo Window", &showDemoWindow)      // Edit bools storing our window open/close state
			imgui.SliderInt("列数", &imageColumns, 1, 10)
			if !load {
				load = true
				go func() {
					pointer := images
					mutex.Lock()
					for i := 0; i < len(pointer); i++ {
						if stoploading {
							break
						}
						loadImageFile(&pointer[i])
					}
					mutex.Unlock()
				}()
			}


				imgui.Columns(int(imageColumns), "mycolumns3")
				mwidth := size[0] / float32(imageColumns)
				for i := 0; i < len(images); i++ {
					if images[i].handle == 0 && images[i].rgba != nil {
						loadImageTexture(&images[i])
					}
					if images[i].width == 0 {
						continue
					} else {
						width := Min(mwidth, float32(images[i].width))
						height := width / float32(images[i].width) * float32(images[i].height)
						if imgui.ImageButton(imgui.TextureID(images[i].handle), imgui.Vec2{width, height}) {
							enlargeImageIndex = i
							imgui.OpenPopup("imagePop")
						}
						imgui.NextColumn()
					}
				}
				imgui.Columns(1, "")

				if imgui.BeginPopupModalV("imagePop",nil,imgui.WindowFlagsNoTitleBar|imgui.WindowFlagsAlwaysAutoResize) {
					if imgui.ImageButton(imgui.TextureID(images[enlargeImageIndex].handle), imgui.Vec2{float32(images[enlargeImageIndex].width), float32(images[enlargeImageIndex].height)}){
						imgui.CloseCurrentPopup()
					}
					imgui.EndPopup()
				}


			//imgui.Checkbox("Another Window", &showAnotherWindow)

			//imgui.SliderFloat("float", &f, 0.0, 1.0) // Edit one float using a slider from 0.0f to 1.0f
			// TODO add example of ColorEdit3 for clearColor

			//if imgui.Button("Button") { // Buttons return true when clicked (most widgets return true when edited/activated)
			//	counter++
			//}
			//imgui.SameLine()
			//imgui.Text(fmt.Sprintf("counter = %d", counter))

			// TODO add text of FPS based on IO.Framerate()

			imgui.End()
		}

		// 3. Show another simple window.
		if showAnotherWindow {
			// Pass a pointer to our bool variable (the window will have a closing button that will clear the bool when clicked)
			imgui.BeginV("Another window", &showAnotherWindow, 0)

			imgui.Text("Hello from another window!")
			if imgui.Button("Close Me") {
				showAnotherWindow = false
			}
			imgui.End()
		}

		// Rendering
		imgui.Render() // This call only creates the draw data list. Actual rendering to framebuffer is done below.

		r.PreRender(clearColor)
		// A this point, the application could perform its own rendering...
		// app.RenderScene()

		r.Render(p.DisplaySize(), p.FramebufferSize(), imgui.RenderedDrawData())
		p.PostRender()

		// sleep to avoid 100% CPU usage for this demo
		<-time.After(time.Millisecond * 25)
	}
}

func loadImageTexture(imageInfo *ImageInfo) {
	gl.GenTextures(1, &imageInfo.handle)
	target := uint32(gl.TEXTURE_2D)

	gl.BindTexture(target, imageInfo.handle)
	//gl.PixelStorei(gl.UNPACK_ROW_LENGTH, 0)

	gl.TexParameteri(target, gl.TEXTURE_MIN_FILTER, gl.LINEAR)    // minification filter
	gl.TexParameteri(target, gl.TEXTURE_MAG_FILTER, gl.LINEAR)    // magnification filter
	gl.TexParameteri(target, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE) // magnification filter
	gl.TexParameteri(target, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE) // magnification filter

	gl.TexImage2D(target, 0, gl.RGBA, imageInfo.width, imageInfo.height, 0, gl.RGBA, gl.UNSIGNED_BYTE, gl.Ptr(imageInfo.rgba.Pix))
	gl.GenerateMipmap(imageInfo.handle)
	gl.BindTexture(target, 0)
	imageInfo.rgba = nil
}

func loadImageFile(imageInfo *ImageInfo) {
	if imageInfo.width > 0 {
		return
	}
	img_file, err := os.Open(imageInfo.filePath)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer img_file.Close()
	var img image.Image
	filePath:=strings.ToLower(imageInfo.filePath)
	if strings.HasSuffix(filePath, ".jpg") {
		img, err = jpeg.Decode(img_file)
	} else if strings.HasSuffix(filePath, ".png") {
		img, err = png.Decode(img_file)
	} else if strings.HasSuffix(filePath, ".bmp") {
		img, _, err = image.Decode(img_file)
	} else {
		return
	}
	if err!=nil{
		fmt.Println(err)
		return
	}

	rgba := image.NewRGBA(img.Bounds())
	draw.Draw(rgba, rgba.Bounds(), img, image.ZP, draw.Src)
	imageInfo.width = int32(rgba.Rect.Dx())
	imageInfo.height = int32(rgba.Rect.Dy())
	imageInfo.rgba = rgba
}

func Min(f1, f2 float32) float32 {
	if f1 < f2 {
		return f1
	}
	return f2
}
