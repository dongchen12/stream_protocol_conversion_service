package main

import (
	"bytes"
	"log"
	"time"

	"github.com/deepch/vdk/format/ts"

	"github.com/gin-gonic/gin"
)

func serveHTTP() {
	router := gin.Default()
	gin.SetMode(gin.DebugMode)
	router.GET("/hls/:suuid/index.m3u8", PlayHLS)             // 请求m3u8索引文件
	router.GET("/hls/:suuid/segment/:seq/file.ts", PlayHLSTS) // 请求ts碎片文件
	//router.StaticFS("/static", http.Dir("web/static"))
	err := router.Run(Config.Server.HTTPPort)
	if err != nil {
		log.Fatalln(err)
	}
}

// PlayHLS 从index.m3u8中读取索引信息
func PlayHLS(c *gin.Context) {
	suuid := c.Param("suuid")
	if !Config.ext(suuid) {
		return
	}
	Config.RunIFNotRun(suuid)
	for i := 0; i < 40; i++ {
		index, seq, err := Config.StreamHLSm3u8(suuid) // 获取m3u8的内容
		//log.Println("The content of the m3u8 " + index)
		if err != nil {
			log.Println(err)
			return
		}
		if seq >= 6 {
			_, err := c.Writer.Write([]byte(index)) // 写的东西其实就是刚才那里返回的m3u8的文本内容
			if err != nil {
				log.Println(err)
				return
			}
			return
		}
		log.Println("Play list not ready wait or try update page")
		time.Sleep(1 * time.Second)
	}
}

// PlayHLSTS 给用户发送ts片段
// 这一部分主要是把ts文件写成一个包, 需要维护包头, 包尾等部分
func PlayHLSTS(c *gin.Context) {
	suuid := c.Param("suuid")
	if !Config.ext(suuid) {
		return
	}
	codecs := Config.coGe(c.Param("suuid"))
	if codecs == nil {
		return
	}
	outfile := bytes.NewBuffer([]byte{})
	Muxer := ts.NewMuxer(outfile)
	err := Muxer.WriteHeader(codecs)
	if err != nil {
		log.Println(err)
		return
	}
	Muxer.PaddingToMakeCounterCont = true
	seqData, err := Config.StreamHLSTS(c.Param("suuid"), stringToInt(c.Param("seq")))
	if err != nil {
		log.Println(err)
		return
	}
	if len(seqData) == 0 {
		log.Println(err)
		return
	}
	for _, v := range seqData {
		v.CompositionTime = 1
		err = Muxer.WritePacket(*v)
		if err != nil {
			log.Println(err)
			return
		}
	}
	err = Muxer.WriteTrailer()
	// 上面的部分使用Muxer将必要的packet部分写入outfile
	// 下面的部分负责将写好的packet的raw data发送到用户
	if err != nil {
		log.Println(err)
		return
	}
	_, err = c.Writer.Write(outfile.Bytes())
	if err != nil {
		log.Println(err)
		return
	}
}
