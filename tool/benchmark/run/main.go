package main

import (
	"fmt"
	"log"
	"math/rand"
	. "snow/common"
	"snow/internal/broadcast"
	"snow/internal/plumtree"
	"snow/tool"
	"time"
)

func main() {

	//测试轮数
	rounds := 10
	//for i := 2; i <= 8; i = i + 2 {
	//	benchmark(600, i, rounds)
	//}
	for i := 100; i <= 600; i = i + 100 {
		//因为在上一组的测试里已经测过了
		benchmark(i, 4, rounds)
	}
	// 主线程保持运行
	select {}
}
func benchmark(n int, k int, rounds int) {
	var err error
	configPath := "E:\\code\\go\\Snow\\config\\config.yml"
	//消息大小
	strLen := 100

	initPort := 40000
	testMode := []MsgType{RegularMsg, EagerPush, ColoringMsg, GossipMsg} //按数组中的顺序决定跑的时候的顺序
	serversAddresses := initAddress(n, initPort)
	tool.Num = n
	tool.InitPort = initPort
	msg := randomByteArray(strLen)
	serverList := make([]*plumtree.Server, 0)

	for i := 0; i < n; i++ {
		action := createAction(i + 1)
		f := func(config *broadcast.Config) {
			config.Port = initPort + i
			config.FanOut = k
			config.DefaultServer = serversAddresses
		}
		config, err := broadcast.NewConfig(configPath, f)
		if err != nil {
			panic(err)
			return
		}
		server, err := plumtree.NewServer(config, action)
		if err != nil {
			log.Println(err)
			return
		}
		serverList = append(serverList, server)
	}

	defer func() {
		time.Sleep(5 * time.Second)
		for _, v := range serverList {
			v.Close()
		}
	}()
	//节点启动完之后再跑
	time.Sleep(time.Duration(n/200) * time.Second)
	//portCounter := 0
	for _, mode := range testMode {

		for i := range rounds {
			//serverListTemp := make([]*plumtree.Server, 0)
			//for i := 0; i < (n / 100); i++ {
			//	action := createAction(i + 1)
			//	f := func(config *broadcast.Config) {
			//		portCounter++
			//		config.Port = initPort + n + 1 + portCounter
			//		config.FanOut = k
			//		config.DefaultServer = serversAddresses
			//		config.Report = true
			//	}
			//	config, err := broadcast.NewConfig(configPath, f)
			//	if err != nil {
			//		panic(err)
			//		return
			//	}
			//	server, err := plumtree.NewServer(config, action)
			//	if err != nil {
			//		log.Println(err)
			//		return
			//	}
			//	server.ApplyJoin(server.Config.InitialServer)
			//	serverListTemp = append(serverListTemp, server)
			//}

			// 1秒一轮,节点可能还没有离开新的广播就发出了	4秒足够把消息广播到所有节点
			fmt.Printf("=== %d =====\n", i)
			time.Sleep(1 * time.Second)
			if mode == RegularMsg {
				err = serverList[0].RegularMessage(msg, UserMsg)
			} else if mode == ColoringMsg {
				err = serverList[0].ColoringMessage(msg, UserMsg)
			} else if mode == GossipMsg {
				err = serverList[0].GossipMessage(msg, UserMsg)
			} else if mode == EagerPush {
				serverList[0].PlumTreeBroadcast(msg, UserMsg)
			}
			if err != nil {
				log.Println("Error broadcasting message:", err)
			}

			//for _, v := range serverListTemp {
			//	v.ApplyLeave()
			//}

			//放一个新的

		}

	}
}

// 编号从0开始
func createAction(num int) broadcast.Action {
	syncAction := func(bytes []byte) bool {
		s := string(bytes)
		//随机睡眠时间，百分之5的节点是掉队者节点
		if num%20 == 0 {
			time.Sleep(1 * time.Second)
		} else {
			randInt := tool.RandInt(10, 200)
			time.Sleep(time.Duration(randInt) * time.Millisecond)
		}

		fmt.Println("这里是同步处理消息的逻辑：", s)
		return true
	}
	asyncAction := func(bytes []byte) {
		//s := string(bytes)
		//fmt.Println("这里是异步处理消息的逻辑：", s)
	}
	reliableCallback := func(isConverged bool) {
		fmt.Println("这里是：可靠消息回调------------------------------", isConverged)
	}
	action := broadcast.Action{
		SyncAction:       &syncAction,
		AsyncAction:      &asyncAction,
		ReliableCallback: &reliableCallback,
	}
	return action
}
func randomByteArray(length int) []byte {
	rand.Seed(time.Now().UnixNano()) // 设置随机种子
	bytes := make([]byte, length)

	for i := range bytes {
		bytes[i] = byte(rand.Intn(26) + 'a') // 生成 a-z 之间的随机字符
	}

	return bytes
}
func initAddress(n int, port int) []string {
	strings := make([]string, 0)
	for i := 0; i < n; i++ {
		addr := fmt.Sprintf("127.0.0.1:%d", port+i)
		strings = append(strings, addr)
	}
	return strings
}
