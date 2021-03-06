package netgate

import (
	"gonet/actor"
	"gonet/base"
	"gonet/message"
	"gonet/network"
	"gonet/rpc"
	"strings"
)

var(
	C_A_LoginRequest = strings.ToLower("C_A_LoginRequest")
	C_A_RegisterRequest 	 = strings.ToLower("C_A_RegisterRequest")
)

type(
	UserPrcoess struct{
		actor.Actor
	}

	IUserPrcoess interface {
		actor.IActor

		CheckClientEx(uint32, string, rpc.RpcHead)bool
		CheckClient(uint32, string,  rpc.RpcHead) *AccountInfo
		SwtichSendToWorld(uint32, string, rpc.RpcHead, []byte)
		SwtichSendToAccount(uint32, string,  rpc.RpcHead, []byte)
		SwtichSendToZone(uint32, string,  rpc.RpcHead, []byte)
	}
)

func (this *UserPrcoess) CheckClientEx(sockId uint32, packetName string, head rpc.RpcHead) bool{
	if IsCheckClient(packetName){
		return  true
	}

	accountId := SERVER.GetPlayerMgr().GetAccount(sockId)
	if accountId <= 0 || accountId != head.Id {
		SERVER.GetLog().Fatalf("Old socket communication or viciousness[%d].", sockId)
		return false
	}
	return  true
}

func (this *UserPrcoess) CheckClient(sockId uint32, packetName string, head rpc.RpcHead) *AccountInfo{
	pAccountInfo := SERVER.GetPlayerMgr().GetAccountInfo(sockId)
	if pAccountInfo != nil && (pAccountInfo.AccountId <= 0 || pAccountInfo.AccountId != head.Id){
		SERVER.GetLog().Fatalf("Old socket communication or viciousness[%d].", sockId)
		return nil
	}
	return pAccountInfo
}

func (this *UserPrcoess) SwtichSendToWorld(socketId uint32, packetName string, head rpc.RpcHead, buff []byte){
	pAccountInfo := this.CheckClient(socketId, packetName, head)
	if pAccountInfo != nil{
		buff = base.SetTcpEnd(buff)
		head.ClusterId = pAccountInfo.WClusterId
		SERVER.GetWorldCluster().Send(head, buff)
	}
}

func (this *UserPrcoess) SwtichSendToAccount(socketId uint32, packetName string, head rpc.RpcHead, buff []byte){
	if this.CheckClientEx(socketId, packetName, head) == true {
		buff = base.SetTcpEnd(buff)
		head.SendType = message.SEND_BALANCE
		SERVER.GetAccountCluster().Send(head, buff)
	}
}

func (this *UserPrcoess) SwtichSendToZone(socketId uint32, packetName string, head rpc.RpcHead, buff []byte){
	pAccountInfo := this.CheckClient(socketId, packetName, head)
	if pAccountInfo != nil{
		buff = base.SetTcpEnd(buff)
		head.ClusterId = pAccountInfo.ZClusterId
		SERVER.GetZoneCluster().Send(head, buff)
	}
}

func (this *UserPrcoess) PacketFunc(socketid uint32, buff []byte) bool{
	packetId, data := message.Decode(buff)
	packet := message.GetPakcet(packetId)
	if packet == nil{
		//客户端主动断开
		if packetId == network.DISCONNECTINT{
			stream := base.NewBitStream(buff, len(buff))
			stream.ReadInt(32)
			SERVER.GetPlayerMgr().SendMsg(rpc.RpcHead{},"DEL_ACCOUNT", uint32(stream.ReadInt(32)))
		}else{
			SERVER.GetLog().Printf("包解析错误1  socket=%d", socketid)
		}
		return true
	}

	//获取配置的路由地址
	destServerType := packet.(message.Packet).GetPacketHead().DestServerType
	err := message.UnmarshalText(packet, data)
	if err != nil{
		SERVER.GetLog().Printf("包解析错误2  socket=%d", socketid)
		return true
	}

	packetHead := packet.(message.Packet).GetPacketHead()
	packetHead.DestServerType = destServerType
	if packetHead == nil || packetHead.Ckx != message.Default_Ipacket_Ckx || packetHead.Stx != message.Default_Ipacket_Stx {
		SERVER.GetLog().Printf("(A)致命的越界包,已经被忽略 socket=%d", socketid)
		return true
	}

	packetName := message.GetMessageName(packet)
	head := rpc.RpcHead{Id:packetHead.Id}
	if packetName  == C_A_LoginRequest{
		head.ClusterId = socketid
	}else if packetName  == C_A_RegisterRequest {
		head.ClusterId = socketid
	}

	//解析整个包
	if packetHead.DestServerType == message.SERVICE_WORLDSERVER{
		this.SwtichSendToWorld(socketid, packetName, head, rpc.Marshal(head, packetName,packet))
	}else if packetHead.DestServerType == message.SERVICE_ACCOUNTSERVER{
		this.SwtichSendToAccount(socketid, packetName, head, rpc.Marshal(head, packetName,packet))
	}else if packetHead.DestServerType == message.SERVICE_ZONESERVER{
		this.SwtichSendToZone(socketid, packetName, head, rpc.Marshal(head, packetName, packet))
	}else{
		this.Actor.PacketFunc(socketid, rpc.Marshal(head, packetName, packet))
	}

	return true
}

func (this *UserPrcoess) Init(num int) {
	this.Actor.Init(num)
	this.RegisterCall("C_G_LogoutRequest", func(accountId int, UID int){
		SERVER.GetLog().Printf("logout Socket:%d Account:%d UID:%d ",this.GetSocketId(), accountId,UID )
		SERVER.GetPlayerMgr().SendMsg(rpc.RpcHead{},"DEL_ACCOUNT", this.GetSocketId())
		SendToClient(this.GetSocketId(), &message.C_G_LogoutResponse{PacketHead:message.BuildPacketHead( 0, 0)})
	})

	this.Actor.Start()
}