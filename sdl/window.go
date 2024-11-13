package sdl

import (
	"fmt"
	"unsafe"

	"uk.ac.bris.cs/gameoflife/util"

	"github.com/veandco/go-sdl2/sdl"
)

// Window 定义了一个窗口对象，包含了窗口的尺寸、渲染器、纹理等属性
type Window struct {
	Width, Height int32         // 窗口的宽度和高度
	window        *sdl.Window   // SDL2 窗口对象
	renderer      *sdl.Renderer // 渲染器，用于将图形绘制到窗口上
	texture       *sdl.Texture  // 纹理，作为图像数据的载体
	pixels        []byte        // 存储像素数据的字节切片
}

// filterEvent 是事件过滤函数，用于筛选键盘按下和退出事件
func filterEvent(e sdl.Event, userdata interface{}) bool {
	return e.GetType() == sdl.KEYDOWN || e.GetType() == sdl.QUIT
}

// NewWindow 创建一个新的窗口，并返回该窗口对象
func NewWindow(width, height int32) *Window {
	// 初始化 SDL2 库，必须先调用此函数才能使用 SDL 功能
	err := sdl.Init(sdl.INIT_EVERYTHING)
	util.Check(err) // 错误检查

	// 创建一个 SDL2 窗口，设置窗口标题、位置和尺寸
	window, err := sdl.CreateWindow("GOL GUI", sdl.WINDOWPOS_CENTERED, sdl.WINDOWPOS_CENTERED, width, height, sdl.WINDOW_SHOWN)
	util.Check(err) // 错误检查

	// 创建渲染器，指定窗口作为渲染目标
	renderer, err := sdl.CreateRenderer(window, -1, sdl.WINDOW_SHOWN)
	util.Check(err) // 错误检查

	// 设置渲染器的缩放质量为线性插值（提高图像质量）
	sdl.SetHint(sdl.HINT_RENDER_SCALE_QUALITY, "linear")

	// 设置渲染器的逻辑尺寸，影响渲染的目标尺寸
	err = renderer.SetLogicalSize(width, height)
	util.Check(err) // 错误检查

	// 创建一个纹理，指定纹理的像素格式为 ARGB8888
	texture, err := renderer.CreateTexture(sdl.PIXELFORMAT_ARGB8888, sdl.TEXTUREACCESS_STATIC, width, height)
	util.Check(err) // 错误检查

	// 设置事件过滤器，只处理键盘按下和退出事件
	sdl.SetEventFilterFunc(filterEvent, nil)

	// 返回创建的窗口对象
	return &Window{
		width,
		height,
		window,
		renderer,
		texture,
		make([]byte, width*height*4), // 初始化像素数据数组，每个像素用 4 字节表示 RGBA
	}
}

// Destroy 销毁窗口对象及其关联的资源
func (w *Window) Destroy() {
	// 销毁纹理、渲染器和窗口
	err := w.texture.Destroy()
	util.Check(err) // 错误检查
	err = w.renderer.Destroy()
	util.Check(err) // 错误检查
	err = w.window.Destroy()
	util.Check(err) // 错误检查

	// 退出 SDL2
	sdl.Quit()
}

// RenderFrame 渲染一个新帧
func (w *Window) RenderFrame() {
	// 更新纹理的数据，使用像素数据数组
	err := w.texture.Update(nil, unsafe.Pointer(&w.pixels[0]), int(w.Width*4))
	util.Check(err) // 错误检查

	// 清空渲染器，准备下一帧渲染
	err = w.renderer.Clear()
	util.Check(err) // 错误检查

	// 将纹理复制到渲染器上，渲染到窗口
	err = w.renderer.Copy(w.texture, nil, nil)
	util.Check(err) // 错误检查

	// 更新渲染器，显示渲染结果
	w.renderer.Present()
}

// PollEvent 轮询事件并返回一个 SDL 事件
func (w *Window) PollEvent() sdl.Event {
	// 返回一个事件，如果没有事件则返回 nil
	return sdl.PollEvent()
}

// SetPixel 设置指定坐标的像素为白色（完全不透明的白色）
func (w *Window) SetPixel(x, y int) {
	// 获取窗口的宽度
	width := int(w.Width)

	// 根据给定的 (x, y) 坐标计算像素在像素数据数组中的位置
	// 每个像素占用 4 字节（RGBA），所以需要乘以 4
	w.pixels[4*(y*width+x)+0] = 0xFF // R 通道设为最大值（白色）
	w.pixels[4*(y*width+x)+1] = 0xFF // G 通道设为最大值（白色）
	w.pixels[4*(y*width+x)+2] = 0xFF // B 通道设为最大值（白色）
	w.pixels[4*(y*width+x)+3] = 0xFF // A 通道设为最大值（完全不透明）
}

// FlipPixel 翻转指定坐标的像素（如果是白色则变黑，反之亦然）
func (w *Window) FlipPixel(x, y int) {
	// 检查坐标是否在窗口范围内
	if x < 0 || y < 0 || x >= int(w.Width) || y >= int(w.Height) {
		// 如果超出范围，则抛出错误
		panic(fmt.Sprintf("CellFlipped event at (%d, %d) is outside the bounds of the window.", x, y))
	}

	// 获取窗口的宽度
	width := int(w.Width)

	// 计算该像素的在像素数据数组中的位置
	// 按位取反每个通道的值，从而翻转颜色（白色变黑，反之亦然）
	w.pixels[4*(y*width+x)+0] = ^w.pixels[4*(y*width+x)+0] // 取反 R 通道
	w.pixels[4*(y*width+x)+1] = ^w.pixels[4*(y*width+x)+1] // 取反 G 通道
	w.pixels[4*(y*width+x)+2] = ^w.pixels[4*(y*width+x)+2] // 取反 B 通道
	w.pixels[4*(y*width+x)+3] = ^w.pixels[4*(y*width+x)+3] // 取反 A 通道
}

// CountPixels 统计白色像素的数量
func (w *Window) CountPixels() int {
	count := 0
	// 遍历所有像素，每次跳过 4 字节（代表一个像素的 RGBA 四个通道）
	for i := 0; i < int(w.Width)*int(w.Height)*4; i += 4 {
		// 如果 R 通道的值为 0xFF，说明该像素是白色的
		if w.pixels[i] == 0xFF {
			count++ // 统计白色像素
		}
	}
	return count
}

// ClearPixels 清空所有像素，将其设置为黑色
func (w *Window) ClearPixels() {
	// 将所有像素的字节数据设置为 0，表示黑色（RGBA: 0, 0, 0, 0）
	for i := range w.pixels {
		w.pixels[i] = 0
	}
}
