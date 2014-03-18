package network

import(
    "fmt"
    "net"
    "os"
    "encoding/gob"
    "container/list"
    "strings"
    "time"              //This file is for the sleep time
    "runtime"           //Used for printing the line on the console
    ".././Server"       //Library for defining ElementQueue
)

const PORT_STATUS = ":20019"
const PORT_CMD  = ":20018"
const PORT_HEART_BIT = ":20100"
//Broadcast lab 129.241.187.255

// Do not use '0' in the Message struct for avoiding problems with the decoder
const STATUS    = 1
const START     = 2
const CMD       = 3
const ACK       = 4
const LAST_ACK  = 5

const DEBUG = true
const LAYOUT_TIME = "15:04:05.000"

type Message struct{
    IDsender string
    IDreceiver string
    MsgType byte
    SizeGotoQueue int
    GotoQueue [] server.ElementQueue
    ActualPos int
}

var LocalIP string

func NetworkManager(ChanToDecision chan Message,ChanFromDecision chan Message,ChanToRedun chan Message,ChanFromRedun chan Message, ChanToServer chan<- server.ServerMsg){

	var MyPID int
	MyPID = os.Getpid()

    fmt.Println("NET_ Network Manager started")

// UPD status
	//Address from where we are going to listen for others status messages
	LocalAddrStatus,err := net.ResolveUDPAddr("udp4",PORT_STATUS)
	check(err)

    //Address to where we are going to send our status(BROADCAST)
	RemoteAddrStatus,err := net.ResolveUDPAddr("udp4","129.241.187.255"+PORT_STATUS)
	check(err)

	// Make connection for sending status
	ConnStatusSend,err := net.DialUDP("udp4",nil,RemoteAddrStatus)
	check(err)

	// Create connection for listening (used for receive broadcast messages)
	ConnStatusListen,err := net.ListenUDP("udp4",LocalAddrStatus)

	// find out own IP address
	LocalIPaddr := ConnStatusSend.LocalAddr()
	LocalIPtmp := strings.SplitN(LocalIPaddr.String(),":",2)
	LocalIP = LocalIPtmp[0]

// UDP command
	//Address from where we are going to listen to others Command messages
	LocalAddrCmd,err := net.ResolveUDPAddr("udp4",PORT_CMD)
	check(err)

    // connection for listening
    ConnCmd,err := net.ListenUDP("udp4",LocalAddrCmd)

    if(DEBUG){
        _,file,line,_ := runtime.Caller(0)
        fmt.Println(file, line)
    }

// UDP alive connection for backup program
	//Resolve address to send, in this case our own address
	LoopbackAlive,err := net.ResolveUDPAddr("udp4","127.0.0.1"+PORT_HEART_BIT)
	check(err)

	//Make connection for sending the loopback message
	ConnAliveSend,err := net.DialUDP("udp4",nil,LoopbackAlive)
	check(err)

//Create go routines
    go ListenerStatus(ConnStatusListen,ChanToRedun)
    go ListenerCmd(ConnCmd,ChanToDecision)
    go SenderStatus(ConnStatusSend,ChanFromRedun)
    go SenderCmd(ChanFromDecision)

    //Do nothing so that go routines are not terminated
    for {
        SenderAlive(ConnAliveSend, MyPID, ChanToServer)
        time.Sleep(1000*time.Millisecond)
    }
}

func SenderAlive(ConnAlive *net.UDPConn, PID int, ChanToServer chan<- server.ServerMsg){

	var MsgToServer server.ServerMsg
    ChanToServer_Network_Queue := make(chan *list.List)
    ChanToServer_Network_ElementQueue := make(chan server.ElementQueue)

    var GotoQueue *list.List
    var dummyActualPos server.ElementQueue

    var AliveNetwork Message

	//Read the go to queue
    MsgToServer.Cmd = server.CMD_READ_ALL
    MsgToServer.QueueID = server.ID_GOTOQUEUE
    MsgToServer.ChanVal = nil
    MsgToServer.ChanQueue = ChanToServer_Network_Queue

    ChanToServer <- MsgToServer
    GotoQueue =<- ChanToServer_Network_Queue

	//Read the actual position
    MsgToServer.Cmd = server.CMD_READ_ALL
    MsgToServer.QueueID = server.ID_ACTUAL_POS
    MsgToServer.ChanVal = ChanToServer_Network_ElementQueue
    MsgToServer.ChanQueue = nil

    ChanToServer <- MsgToServer
    dummyActualPos =<- ChanToServer_Network_ElementQueue
    dummyActualPos.Direction = server.NONE

	//Reset message
	AliveNetwork = Message{}

	//Add the actual position to the front of the GotoQueue so the new instance goes to the last floor the elevator was
	GotoQueue.PushFront(dummyActualPos)

	AliveNetwork.IDsender = "dummy"  //Filled out by network module
    AliveNetwork.IDreceiver = "dummy"
    AliveNetwork.MsgType = 0
    AliveNetwork.GotoQueue = listToArray(GotoQueue)
    AliveNetwork.ActualPos = PID

    if(DEBUG){fmt.Println("NET_ Before alive message", AliveNetwork)}

	//Create encoder
	enc := gob.NewEncoder(ConnAlive)
	//Send encoded message on connection
	err := enc.Encode(AliveNetwork)
	check(err)
	if(DEBUG){fmt.Println("NET_ Send alive message")}
}

func ListenerStatus(conn *net.UDPConn,Channel chan<- Message){
    var MsgRecv Message
    for {
            //Reset the message because the decoder can not handle values ZERO
            MsgRecv = Message {}
            //Create decoder
            dec := gob.NewDecoder(conn)
            //Receive message on connection
            err := dec.Decode(&MsgRecv)
            check(err)

			//Check if the message you recevied it from you
			if(MsgRecv.IDsender == LocalIP){
				MsgRecv.IDsender = "Local"
			}

            if(DEBUG){fmt.Println("NET_ RecvStatus:",MsgRecv, time.Now())}

            //Discard message if not status
            //Even if it is your local IP send it to the redundancy so it adds it to the participants table
            if(MsgRecv.MsgType == STATUS && err == nil){
                Channel <-MsgRecv
            }
    }
}

func ListenerCmd(conn *net.UDPConn,Channel chan<- Message){
    var MsgRecv Message
    for {
            //Reset the message because the decoder can not handle values ZERO
            MsgRecv = Message {}
            //Create decoder
            dec := gob.NewDecoder(conn)
            //Receive message on connection
            err := dec.Decode(&MsgRecv)
            check(err)

            if(DEBUG){ fmt.Println("NET_ RecvCmd:",MsgRecv, time.Now()) }

            // Discard message if not command related
            if((MsgRecv.MsgType == START || MsgRecv.MsgType == CMD || MsgRecv.MsgType == ACK || MsgRecv.MsgType == LAST_ACK) && err == nil){
                Channel <-MsgRecv
            }
    }
}

func SenderStatus(ConnStatus *net.UDPConn, Channel <-chan Message){
    for{
        var MsgSend Message
        MsgSend = <-Channel
        MsgSend.IDsender = LocalIP;

        //Create encoder
        enc := gob.NewEncoder(ConnStatus)
        //Send encoded message on connection
        err := enc.Encode(MsgSend)
        if(DEBUG){
            fmt.Println(err)
        }
        check(err)
        if(DEBUG){fmt.Println("NET_ StatusSent:",MsgSend, time.Now())}
    }
}

func SenderCmd(Channel <-chan Message){
    for{
        var MsgSend Message
        MsgSend =<-Channel
        MsgSend.IDsender = LocalIP;

        RemoteAddrCmd,err := net.ResolveUDPAddr("udp4",MsgSend.IDreceiver+PORT_CMD)
        check(err)

        // Make connection for sending
        ConnCmd,err := net.DialUDP("udp4",nil,RemoteAddrCmd)
        check(err)

        //Create encoder
        enc := gob.NewEncoder(ConnCmd)
        //Send encoded message on connection
        err = enc.Encode(MsgSend)
        check(err)

        //Close connection
        ConnCmd.Close()

        if(DEBUG){fmt.Println("NET_ CmdSent   :",MsgSend, time.Now())}
    }
}

func listToArray(Queue *list.List) [] server.ElementQueue {
    var index int = 0
    buf := make([] server.ElementQueue, Queue.Len())

    for e := Queue.Front(); e != nil; e = e.Next(){
        buf[index] = e.Value.(server.ElementQueue)
        index++
    }
    return buf
}

func check(err error){
    if err != nil{
        fmt.Fprintf(os.Stderr,time.Now().Format(LAYOUT_TIME))
        fmt.Fprintf(os.Stderr,"NET_  Error: %s\n",err.Error())
    }
}