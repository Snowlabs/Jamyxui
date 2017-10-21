package main

// // #cgo pkg-config: gtk+-3.0
// // #include <gtk/gtk.h>
// import "C"

// #include <gtk/gtk.h>
// #cgo pkg-config: gtk+-2.0
import "C"

import (
    "unsafe"
    "time"
    "fmt"
    "bufio"
    "os"
    "log"
    // "math"
    "github.com/xthexder/go-jack"
    "github.com/mattn/go-gtk/gdk"
    "github.com/mattn/go-gtk/gtk"
    "github.com/mattn/go-gtk/glib"
    "github.com/Snowlabs/jamyxgo"
)


func interactiveLoop(target *jamyxgo2.Target) {
    reader := bufio.NewReader(os.Stdin)
    for {
        fmt.Print("Command: ")
        reader.ReadString('\n')
        // cmd, _ := reader.ReadString('\n')
        // log.Println(target.SendCommand(cmd))
    }
}

type gtkMeter  struct { *gtk.DrawingArea }
type gdkPixmap struct { *gdk.Pixmap }

func (pm *gdkPixmap) Fill(gc *gdk.GC) {
    pm.GetDrawable().DrawRectangle(gc, true, 0, 0, -1, -1)
}

type Meter struct {
    PortName string
    IsMono bool
    PortL *jack.Port
    PortR *jack.Port
    PortM *jack.Port
    MeterGtk *gtkMeter
    MeterValueL *float32
    MeterValueR *float32
    MeterValueM *float32
}

func gtkNewMeter(isStereo, isMono bool, meterValues [](*float32)) *gtkMeter {
    // Macro
    invertCoords := func(t_w, t_h, x, y, w, h int) (rx, ry, rw, rh int) {
        rx, ry, rw, rh = x, y, w, h

        rx = t_w - (x + w)
        ry = t_h - (y + h)

        return rx, ry, w, h
    }
    propCoords := func(t_w, t_h int, x, y, w, h float32) (rx, ry, rw, rh int) {
        t_w_f := float32(t_w)
        t_h_f := float32(t_h)

        rx = int(t_w_f * x)
        ry = int(t_h_f * y)
        rw = int(t_w_f * w)
        rh = int(t_h_f * h)

        return rx, ry, rw, rh
    }
    rgb := func(r, g, b float64) (rr, rg, rb uint16) {
        const max float64 = 65535
        return uint16(max*r), uint16(max*g), uint16(max*b)
    }

    meter := gtk.NewDrawingArea()

    var pixmap *gdkPixmap
    var bgGC, barGC, _GC *gdk.GC
    var gdkwin *gdk.Window

    var vm_width, vm_height int

    bgColor  := gdk.NewColorRGB(rgb(0.1, 0.1, 0.2))
    // bgColor, _ := meter.GetStyle().LookupColor("bg_color")
    barColor := gdk.NewColorRGB(rgb(0.2, 0.6, 0.9))
    fmt.Println(barColor, bgColor)

    // Config event
    meter.Connect("configure-event", func(){
        if pixmap != nil { pixmap.Unref() }
        this                := meter
        allocation          := this.GetAllocation()
        vm_width, vm_height  = allocation.Width, allocation.Height

        gdkwin      = this.GetWindow()
        pixmap      = &gdkPixmap{gdk.NewPixmap(gdkwin.GetDrawable(), vm_width, vm_height, 24)}
        bgGC        = gdk.NewGC(pixmap.GetDrawable())
        barGC       = gdk.NewGC(pixmap.GetDrawable())
        _GC         = gdk.NewGC(pixmap.GetDrawable())

        bgGC .SetRgbFgColor(bgColor)
        barGC.SetRgbFgColor(barColor)

        pixmap.Fill(bgGC)
    })

    var curLevelL, curLevelR, curLevelM float32 = 0, 0, 0
    var fallSpeed float32 = 0.04
    // Expose event
    meter.Connect("expose-event", func() {
		if pixmap == nil {
			return
		}
        // Draw bg
        pixmap.Fill(bgGC)

        if isMono {
            curLevelM -= fallSpeed
            if curLevelM < 0 { curLevelM = 0 }
            if *meterValues[0] > curLevelM { curLevelM = *meterValues[2] }

            // Draw bars
            x, y, w, h := propCoords  (vm_width, vm_height, 0.3, 0, 0.50, curLevelM)
            x, y, w, h  = invertCoords(vm_width, vm_height, x  , y, w  , h)
            pixmap.GetDrawable().DrawRectangle(barGC, true, x  , y, w  , h)
        } else {
            curLevelL -= fallSpeed
            curLevelR -= fallSpeed
            if curLevelL < 0 { curLevelL = 0 }
            if curLevelR < 0 { curLevelR = 0 }
            if *meterValues[0] > curLevelL { curLevelL = *meterValues[0] }
            if *meterValues[1] > curLevelR { curLevelR = *meterValues[1] }

            // Draw bars
            x, y, w, h := propCoords  (vm_width, vm_height, 0.55, 0, 0.40, curLevelL)
            x, y, w, h  = invertCoords(vm_width, vm_height, x  , y, w  , h)
            pixmap.GetDrawable().DrawRectangle(barGC, true, x  , y, w  , h)

            x, y, w, h  = propCoords  (vm_width, vm_height, 0.1, 0, 0.35, curLevelR)
            x, y, w, h  = invertCoords(vm_width, vm_height, x, y, w  , h)
            pixmap.GetDrawable().DrawRectangle(barGC, true, x, y, w  , h)
        }


        // Display everything
		gdkwin.GetDrawable().DrawDrawable(_GC, pixmap.GetDrawable(), 0, 0, 0, 0, -1, -1)

        // vol_monitor2.QueueDraw()
	})

    return &gtkMeter{meter}
}

type monitorButton struct {
    GtkButton *gtk.ToggleButton
    Port *jamyxgo2.Port
    CallId int
}

var g_mon_butts = [](*monitorButton){}
func monitorButtonCB(mon_butt *monitorButton, target *jamyxgo2.Target) {

    for _, mb := range g_mon_butts {
        mb.GtkButton.HandlerBlock(mb.CallId)
        mb.GtkButton.SetActive(false)
        mb.GtkButton.HandlerUnblock(mb.CallId)
    }
    mon_butt.GtkButton.HandlerBlock(mon_butt.CallId)
    mon_butt.GtkButton.SetActive(true)
    mon_butt.GtkButton.HandlerUnblock(mon_butt.CallId)

    mon_butt.Port.SetMonitored()
}

func channelWidget(port *jamyxgo2.Port,
                   target *jamyxgo2.Target,
                   jclient **jack.Client,
               ) (widget *gtk.VBox, meter *Meter) {

    // Macro
    getPrecision := func(vol float32) int {
        if vol == 100 { return 1 }
        if vol == 0   { return 3 }
        return 2
    }
    getVolLabelText := func(vol float32) string {
        return fmt.Sprintf("%5.[2]*[1]f", vol, getPrecision(vol))
    }

    // ==== Initialize values ====
    var meterValL float32 = 0.5
    var meterValR float32 = 0.5
    var meterValM float32 = 0.5
    meterValues := [](*float32){ &meterValL, &meterValR, &meterValM }


    // ==== Initialize gtk objects ====
    name_label  := gtk.NewLabel(port.Port)
    vol_label   := gtk.NewLabel(getVolLabelText(port.Vol))
    vol_slider  := gtk.NewVScaleWithRange(0, 100, 1)
    vol_monitor := gtkNewMeter(true, port.IsMono, meterValues)
    vol_frame   := gtk.NewFrame("")
    vol_vbox    := gtk.NewVBox(false, 0)
    vol_hbox    := gtk.NewHBox(true, 0)
    mon_butt    := gtk.NewToggleButtonWithLabel("MON")
    vbox        := gtk.NewVBox(false, 0)

    is_local_change := false

    monButObj := &monitorButton{mon_butt, port, 0}
    g_mon_butts = append(g_mon_butts, monButObj)

    // ==== Configure gtk objects ====
    name_label.SetSizeRequest(0, -1)

    vol_label.SetPadding(3, 3)
    vol_slider.SetDrawValue(false)
    vol_slider.SetValue(float64(port.Vol))
    vol_slider.SetInverted(true)

    moned_port := target.GetMonitorPort()
    if moned_port.IsInput == port.IsInput {
        mon_butt.SetActive(moned_port.Port == port.Port)
    }

    // ==== Place gtk objects ====
    vol_hbox.PackStart(vol_slider,  true,  true,  0)
    vol_hbox.PackStart(vol_monitor, true,  true,  0)

    vol_vbox.PackStart(vol_label,   false, false, 0)
    vol_vbox.PackStart(vol_hbox,    true,  true,  0)

    vol_frame.Add(vol_vbox)

    vbox.PackStart(name_label, false, false, 0)
    vbox.PackStart(vol_frame,  true,  true,  0)

    vbox.PackEnd(mon_butt, false, false, 0)

    // ==== Set callbacks and goroutines ====
    vol_slider.Connect("value_changed", func(){
        is_local_change = true
        vol := float32(vol_slider.GetValue())
        log.Println("VOLUME SLIDER: ", vol)
        port.SetVol(vol)
        vol_label.SetText(getVolLabelText(vol))
    })

    id := mon_butt.Connect("toggled", func(){ monitorButtonCB(monButObj, target) })
    monButObj.CallId = id

    go func() {
        for {
            // This is a blocking call waiting for a change in volume and returning it
            port := target.VolumeListen(port.IsInput, port.Port)
            if is_local_change {
                is_local_change = false
                continue
            }
            vol_slider.SetValue(float64(port.Vol))
            vol_label.SetText(getVolLabelText(port.Vol))
        }
    }()

    // Construct Meter object
    // return vbox, vol_monitor, &vol_meter_val
    suffix := ""; if port.IsInput { suffix="Out "}

    meter             = new(Meter)
    meter.IsMono      = port.IsMono
    meter.PortName    = fmt.Sprintf("Jacon:%s %s", port.Port, suffix)
    if !meter.IsMono {
        meter.PortL       = (*jclient).GetPortByName(meter.PortName+"L")
        meter.PortR       = (*jclient).GetPortByName(meter.PortName+"R")
    } else {
        meter.PortM       = (*jclient).GetPortByName(meter.PortName+"M")
    }
    meter.MeterGtk    = vol_monitor
    meter.MeterValueL = &meterValL
    meter.MeterValueR = &meterValR
    meter.MeterValueM = &meterValM
    return vbox, meter
}

var g_meters [](*Meter)

func jackProcess(nframes uint32) int {
    for _, meter := range g_meters {
        if !meter.IsMono {
            framesL := meter.PortL.GetBuffer(nframes)
            framesR := meter.PortR.GetBuffer(nframes)

            // find peak
            var peakL float32 = 0
            var peakR float32 = 0
            for _, frame := range framesL {
                if float32(frame) > peakL { peakL = float32(frame) }
            }
            for _, frame := range framesR {
                if float32(frame) > peakR { peakR = float32(frame) }
            }
            *meter.MeterValueL = peakL
            *meter.MeterValueR = peakR
        } else {
            framesM := meter.PortM.GetBuffer(nframes)

            // find peak
            var peakM float32 = 0
            for _, frame := range framesM {
                if float32(frame) > peakM { peakM = float32(frame) }
            }
            *meter.MeterValueM = peakM
        }

        gdk.ThreadsEnter()
        meter.MeterGtk.QueueDraw()
        gdk.ThreadsLeave()
    }
    return 0
}

func windowWidget(target *jamyxgo2.Target, jclient **jack.Client) gtk.IWidget {
    hbox := gtk.NewHBox(false, 0)

    all_ports := target.GetPorts()
    inputs  := all_ports.Inputs
    outputs := all_ports.Outputs

    var meters []*Meter

    log.Println("Inputs:", inputs)
    for _, in := range inputs {
        i := in
        chan_w, meter := channelWidget(&i, target, jclient)
        for _, out := range outputs {
            b := gtk.NewToggleButtonWithLabel(out.Port)

            o := out

            b.SetActive(i.IsConnectedToPort(o))
            b.Connect("toggled", func() {
                i.ToggleConnectionWithPort(o)
            })
            b.SetSizeRequest(0, -1)
            chan_w.PackStart(b, false, false, 0)
            chan_w.ReorderChild(b, 1)
        }
        hbox.PackStart(chan_w, false, true, 0)

        meters = append(meters, meter)
    }

    log.Println("Outputs:", outputs)
    for _, out := range outputs {
        o := out
        chan_w, meter := channelWidget(&o, target, jclient)
        hbox.PackEnd(chan_w, false, true, 0)

        meters = append(meters, meter)
    }
    g_meters = meters


    return hbox
}

func setupWindow(target *jamyxgo2.Target, jclient **jack.Client) {
    gdk.ThreadsInit()
    gtk.Init(nil)
    window := gtk.NewWindow(gtk.WINDOW_TOPLEVEL)
	window.SetPosition(gtk.WIN_POS_CENTER)
	window.SetTitle("Jamyxui")
    window.Connect("destroy", gtk.MainQuit)
    window.SetIconFromFile("jamyxui.png")

    menubar := gtk.NewMenuBar()
    aboutBtn := gtk.NewMenuItemWithMnemonic("_About")
    // aboutBtn.Connect("button-release-event", func() { fmt.Println("as"); aboutWin.ShowAll(); })
    openAbout := func() {
        md := gtk.NewAboutDialog()
        md.SetName("Jamyxui")
        md.SetProgramName("Jamyxui")
        md.SetVersion("1.0")
        image := gtk.NewImageFromFile("jamyxui.png")
        md.SetLogo(image.GetPixbuf())
        // md.SetLogoIconName("Icon.png")
        md.SetCopyright("Copyright © 2017 Javier Pollak")
        md.SetAuthors([]string{"Javier Pollak"})
        md.SetArtists([]string{"Javier Pollak"})
        md.SetWebsite("http://github.com/javyre")
        md.SetKeepAbove(true)
        md.SetDecorated(false)
        md.Run()
        md.Destroy()
    }
    aboutBtn.Connect("button-press-event", func(ctx *glib.CallbackContext) bool {
        arg := ctx.Args(0)
        e := *(**gdk.EventButton)(unsafe.Pointer(&arg))
        if e.Button != 1 {
            return false
        }
        openAbout()
        return true
    })
    aboutBtn.Connect("activate", openAbout)
    menubar.Append(aboutBtn)

    vbox := gtk.NewVBox(false, 0)
    vbox.PackStart(menubar, false, false, 0)
    vbox.PackStart(windowWidget(target, jclient), true, true, 0)
    window.Add(vbox)

    window.SetSizeRequest(-1, 300)
    window.ShowAll()
}

var isClientAlive bool = false
func setupJack(target *jamyxgo2.Target) **jack.Client {
    var jclient **jack.Client = new(*jack.Client)

    setup := func () {
        client, _ := jack.ClientOpen("Jamyxui channels monitor", jack.NoStartServer)
        if client == nil {
            log.Println("Could not (re)connect to jack server!")
            isClientAlive = false
            return
        } else { isClientAlive = true }

        client.SetProcessCallback(jackProcess)
        client.OnShutdown(func() { isClientAlive = false })

        if code := client.Activate(); code != 0 { log.Fatal("Failed to activate client!") }

        *jclient = client
    }

    // Reconnection loop
    go func() { for {
        if !isClientAlive {
            fmt.Println("Attempting reconnection to jack server...")
            setup()
        }
        time.Sleep(2*time.Second)
    } } ()

    for !isClientAlive {
        time.Sleep(500*time.Millisecond)
    }

    return jclient
}

func main() {
    target:= jamyxgo2.NewTarget("127.0.0.1", 56065)

    go interactiveLoop(target)

    jclient := setupJack(target)
    defer (*jclient).Close()

    fmt.Println((*jclient).GetPorts("jamyxer:.*", ".*", 0))

    setupWindow(target, jclient)

    gtk.Main()
}
