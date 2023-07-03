package main

import (
	"errors"
	"github.com/deepch/vdk/av"
	"github.com/deepch/vdk/format/rtspv2"
	"log"
	"time"
)

var (
	ErrorStreamExitNoVideoOnStream = errors.New("stream Exit No Video On Stream")
	ErrorStreamExitRtspDisconnect  = errors.New("stream Exit Rtsp Disconnect")
	ErrorStreamExitNoViewer        = errors.New("stream Exit On Demand No Viewer")
)

func serveStreams() {
	for k, v := range Config.Streams {
		go RTSPWorkerLoop(k, v.URL)
	}
}

// RTSPWorkerLoop / 创建一个 RTSP 客户端，如果创建不成功会一致循环创建，直到成功为止
func RTSPWorkerLoop(name, url string) {
	defer Config.RunUnlock(name)
	for {
		log.Println(name, "Stream Try Connect")
		err := RTSPWorker(name, url)
		if err != nil {
			log.Println(err)
		}
		time.Sleep(1 * time.Second) // 每隔1秒试一次
	}
}

func RTSPWorker(name, url string) error {
	// 测试是否有用户链接
	keyTest := time.NewTimer(20 * time.Second)
	var preKeyTS = time.Duration(0)
	var Seq []*av.Packet
	var packetCount int = 0
	// 连接RTSP服务器通过Dial实现, 并且可以通过RTSPClient.Signals获取到一些信号, 通过RTSPClient.OutgoingPacketQueue获取到数据包
	RTSPClient, err := rtspv2.Dial(rtspv2.RTSPClientOptions{URL: url, DisableAudio: false, DialTimeout: 3 * time.Second, ReadWriteTimeout: 3 * time.Second, Debug: false})
	if err != nil {
		return err
	}
	defer RTSPClient.Close() // 确保函数运行结束后关闭 RTSP 客户端
	if RTSPClient.CodecData != nil {
		Config.coAd(name, RTSPClient.CodecData)
	}
	var AudioOnly bool
	// 判断是否是音频流
	if len(RTSPClient.CodecData) == 1 && RTSPClient.CodecData[0].Type().IsAudio() {
		AudioOnly = true
	}
	// 服务器大循环, 从 RTSP 客户端读取数据包, 并且把读取到的packet放进全局的一个相当于context的一个流的列表里面.
	for {
		select { // select 语句会一直等待，直到某个 case 语句完成，然后执行该 case 语句
		case <-keyTest.C:
			return ErrorStreamExitNoVideoOnStream
		case signals := <-RTSPClient.Signals:
			switch signals {
			case rtspv2.SignalCodecUpdate:
				Config.coAd(name, RTSPClient.CodecData)
			case rtspv2.SignalStreamRTPStop:
				return ErrorStreamExitRtspDisconnect
			}
		case packetAV := <-RTSPClient.OutgoingPacketQueue: // 从 RTSP 客户端读取数据包packetAV, 主要逻辑
			packetCount++
			if AudioOnly || packetCount >= 10 {
				keyTest.Reset(20 * time.Second)
				if preKeyTS > 0 {
					Config.StreamHLSAdd(name, Seq, packetAV.Time-preKeyTS)
					packetCount = 0
					Seq = []*av.Packet{}
				}
				preKeyTS = packetAV.Time
			}
			Seq = append(Seq, packetAV)
			// 这么做的话, 一个ts的大小大概缩减为原来的三分之一, 延迟可以大幅降低
		}
	}
}
