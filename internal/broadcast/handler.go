package broadcast

import (
	"bytes"
	"log"
	"net"
	"snow/tool"
)

type MsgType = byte

// 定义枚举值
const (
	pingMsg MsgType = iota
	indirectPingMsg
	ackRespMsg
	suspectMsg
	aliveMsg
	deadMsg
	//节点状态改变
	nodeChange
	//这是消息的发送方式
	coloringMsg
	regularMsg     //普通的消息
	reliableMsg    //可靠消息
	reliableMsgAck //可靠消息回执
	gossipMsg      //gossip消息
)

type MsgAction = byte

const (
	userMsg MsgAction = iota
	applyJoin
	joinStateSync
	nodeJoin
	nodeLeave   //这是自己请求离开的方法
	reportLeave //这是被别人上报的节点离开方法
	regularStateSync
)

func handler(msg []byte, s *Server, conn net.Conn) {
	parentIP := s.Config.GetServerIp(conn.RemoteAddr().String())
	//s.Member.lock.Lock()
	//defer s.Member.lock.Unlock()
	//判断消息类型
	msgType := msg[0]
	//消息要进行的动作
	msgAction := msg[1]
	NodeChange(msg[1:], parentIP, s, conn)
	switch msgType {
	case regularMsg:
		body := s.Config.CutBytes(msg)
		if !isFirst(body, msgType, msgAction, s) {
			return
		}
		if msgAction == nodeJoin {
			//如果不存在
			s.Member.AddMember(s.Config.CutTimestamp(body))
		}
		forward(msg, s, parentIP)
	case coloringMsg:
		body := s.Config.CutBytes(msg)
		if !isFirst(body, msgType, msgAction, s) {
			return
		}
		if msgAction == reportLeave {
			s.Member.RemoveMember(s.Config.CutTimestamp(body))
		}
		forward(msg, s, parentIP)
	case reliableMsg:
		body := s.Config.CutBytes(msg)
		if !isFirst(body, msgType, msgAction, s) {
			return
		}
		//如果自己是叶子节点发送ack给父节点	并删除ack的map
		forward(msg, s, parentIP)
	case reliableMsgAck:
		//ack不需要ActionType
		body := msg[TagLen:]
		//去重的消息可能会过滤掉相同的ack。在消息尾部追加ip来解决
		if !isFirst(body, msgType, msgAction, s) {
			return
		}

		//减少计数器
		s.ReduceReliableTimeout(msg, s.Action.ReliableCallback)
	case gossipMsg:
		//gossip不需要和Snow算法一样携带俩个ip
		body := msg[TagLen:]
		if !isFirst(body, msgType, msgAction, s) {
			return
		}
		data := make([]byte, len(msg))
		copy(data, msg)
		s.SendGossip(data)
	case nodeChange:
		//分别是消息类型，消息时间戳，加入节点的ip
		if !isFirst(msg[1:], msgType, msgAction, s) {
			return
		}

	default:
		log.Printf("Received non type message from %v: %s\n", conn.RemoteAddr(), string(msg))
	}
}

func isFirst(body []byte, msgType MsgType, action MsgAction, s *Server) bool {
	if s.IsReceived(body) && s.Config.ExpirationTime > 0 {
		return false
	}
	if action == userMsg && msgType != reliableMsgAck {
		//如果第二个byte的类型是userMsg才让用户进行处理
		body = s.Config.CutTimestamp(body)
		//这是让用户自己判断消息是否处理过
		if !s.Action.process(body) {
			return false
		}
	}
	return true
}

// 返回自己是不是转发成功，不成功说明是叶子节点
func forward(msg []byte, s *Server, parentIp string) {
	member := make(map[string][]byte)
	msgType := msg[0]
	msgAction := msg[1]
	leftIP := msg[TagLen : s.Config.IpLen()+TagLen]
	rightIP := msg[s.Config.IpLen()+TagLen : s.Config.IpLen()*2+TagLen]
	isLeaf := bytes.Compare(leftIP, rightIP) == 0
	if !isLeaf {
		member, _ = s.NextHopMember(msgType, msgAction, leftIP, rightIP, false)
	}
	//消息中会附带发送给自己的节点
	if msgType == reliableMsg {
		//写入map 以便根据ack进行删除
		b := s.Config.CutBytes(msg)
		hash := []byte(tool.Hash(b))
		if len(member) == 0 {
			//叶子节点 直接发送ack
			//消息内容为2个type，加上当前地址长度+ack长度
			newMsg := make([]byte, 0)
			newMsg = append(newMsg, reliableMsgAck)
			newMsg = append(newMsg, msgAction)
			newMsg = append(newMsg, tool.RandomNumber()...)
			newMsg = append(newMsg, hash...)
			// 叶子节点ip
			newMsg = append(newMsg, s.Config.IPBytes()...)
			//根节点ip
			newMsg = append(newMsg, msg[len(msg)-s.Config.IpLen():]...)
			if msgAction == nodeLeave {
				s.Member.RemoveMember(msg[len(msg)-s.Config.IpLen():])
			}
			s.SendMessage(parentIp, []byte{}, newMsg)
		} else {
			//不是发送节点的化，不需要任何回调
			s.State.AddReliableTimeout(hash, false, len(member), tool.IPv4To6Bytes(parentIp), nil)
		}
		for _, payload := range member {
			payload = append(payload, s.Config.IPBytes()...)
		}

	}

	s.ForwardMessage(msg, member)

}
