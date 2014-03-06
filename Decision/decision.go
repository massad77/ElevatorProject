package decision

import(
    "fmt"
    "time"
    "os"
    "container/list"   //For using lists        
    ".././Network"
    ".././Server"
    ".././Redundancy"
    "strings"
    "strconv"
)

const DEBUG = false
const LAYOUT_TIME = "15:04:05.000"

//States for the Decision module. Go enums
const (
	STANDBY_ST = iota
	MASTER_ST = iota
	SLAVE_ST = iota		
)

//Inner states for the master
const (
	MASTER_1_ST = iota
	MASTER_2_ST = iota
	MASTER_3_ST = iota
	MASTER_4_ST = iota
	MASTER_5_ST = iota
	MASTER_6_ST = iota
)

/*
TODO
Implement the channel to the redundancy for getting the participants table
Not in the parameters list of DecisionManager

Check if at the end we still need the "state" var
*/
func DecisionManager(ChanToServer chan<- server.ServerMsg, ChanFromNetwork <-chan network.Message, ChanToNetwork chan<- network.Message, ChanToRedun chan<- redundancy.TableReqMessage ){

	var MainState int
	MainState = STANDBY_ST
	
	//-----------------NETWORK
	var NetworkMsg network.Message
	
	
	
	//---------------SERVER
    // Channel for receiving data from the server
	ChanToServer_Decision_ElementQueue := make(chan server.ElementQueue)
//    ChanToServer_Decision_Queue := make(chan *list.List)

	var dummyElement server.ElementQueue
	var MsgToServer server.ServerMsg

	fmt.Println("DS_ Decision module started!")
	
	//Tick for check if the ReqQueue is empty
	timeout := time.Tick(200*time.Millisecond)
	for{
		switch(MainState){
			case STANDBY_ST:
				//In standby state, only two options
				select{
				    case NetworkMsg =<- ChanFromNetwork:
				    //Go to slave mode only if you receive a START message
				    if(NetworkMsg.MsgType == network.START){
				    	MainState = SLAVE_ST
				    }
				    
				    case <- timeout:
				    	//Send message to server to know if the Req is empty
				    	// Extract first element from request queue 
						MsgToServer.Cmd = server.CMD_READ_FIRST
						MsgToServer.QueueID = server.ID_REQQUEUE
						MsgToServer.ChanVal = ChanToServer_Decision_ElementQueue
						MsgToServer.ChanQueue = nil              
					   
						ChanToServer <- MsgToServer
						dummyElement =<- ChanToServer_Decision_ElementQueue
				
						if (dummyElement.Floor != -1){
							MainState = MASTER_ST
						}				            
		    }
			
			case MASTER_ST:
				MainState = master(ChanToServer, ChanFromNetwork, ChanToNetwork, ChanToRedun)
			
			case SLAVE_ST:
//				MainState = slave()//EAGM WRITE THE CHANNELS STILL
			
			default:
				fmt.Println("DS_ SOMETHING WENT TERRIBLE WRONG --------MAIN STATE INVALID")
				MainState = STANDBY_ST
		}                   
    }

	
	
	
	for {
		// Extract first element from request queue 
		MsgToServer.Cmd = server.CMD_EXTRACT
		MsgToServer.QueueID = server.ID_REQQUEUE
		MsgToServer.ChanVal = ChanToServer_Decision_ElementQueue
		MsgToServer.ChanQueue = nil              
	   
		ChanToServer <- MsgToServer
		dummyElement =<- ChanToServer_Decision_ElementQueue

		// Add element to the GotoQueue 
		if dummyElement.Floor != -1{
			MsgToServer.Cmd = server.CMD_ADD
			MsgToServer.QueueID = server.ID_GOTOQUEUE
			MsgToServer.Value = dummyElement
			MsgToServer.ChanVal = nil
			MsgToServer.ChanQueue = nil              
	   
			ChanToServer <- MsgToServer
		}
		time.Sleep(100*time.Millisecond)
	}
}

func master (ChanToServer chan<- server.ServerMsg, ChanFromNetwork <-chan network.Message, ChanToNetwork chan<- network.Message, ChanToRedun chan<- redundancy.TableReqMessage) int {

//---------------MASTER
	var MasterST int
	MasterST = MASTER_1_ST
	//TABLE for record who has answer

//-----------------NETWORK
	var MsgToNetwork network.Message
	var MsgFromNetwork network.Message		
	
	
//---------------SERVER
    // Channel for receiving data from the server
//	ChanToServer_Decision_ElementQueue := make(chan server.ElementQueue)
//    ChanToServer_Decision_Queue := make(chan *list.List)

//--------------REDUNDANCY
    ChanToRedun_Dec_Queue := make(chan *list.List)	
    var TableReq redundancy.TableReqMessage
    TableReq.ChanQueue = ChanToRedun_Dec_Queue

	
    var ParticipantsList *list.List
    ParticipantsList = list.New()
	
	for{
		switch(MasterST){
			case MASTER_1_ST:
				//Request the participants table
				ChanToRedun <- TableReq
				ParticipantsList =<- ChanToRedun_Dec_Queue

				//Rest message that will be sent to other elevators
				MsgToNetwork = 	network.Message{}
				//MsgToNetwork.IDsender = "dummy"  //Filled out by network module
				//MsgToNetwork.IDreceiver = "Broadcast"
				MsgToNetwork.MsgType = network.START
				//MsgToNetwork.SizeGotoQueue = 0
				//MsgToNetwork.SizeMoveQueue = 0
				//MsgToNetwork.GotoQueue = buf
				//MsgToNetwork.MoveQueue = buf
				//MsgToNetwork.ActualPos = 0
								
								
				// go through participants table and send a START message throught the network
                for e := ParticipantsList.Front(); e != nil; e = e.Next(){
                	MsgToNetwork.IDreceiver = e.Value.(redundancy.Participant).IPsender
              		ChanToNetwork <- MsgToNetwork
                }

				//Check if you have received a START msg from other elevator
               	timeout := time.Tick(10*time.Millisecond)
               	//Normal ping takes 0.3ms
For_loop_START: for{
                	select{
                		//Wait until yu receive a smaller IP
                		case MsgFromNetwork =<- ChanFromNetwork:                		
                			if(MsgFromNetwork.MsgType == network.START){
                				if(!LocalIPgreater(network.LocalIP,MsgFromNetwork.IDsender)){
                					return SLAVE_ST
                				}								
							}
						case <-timeout:
							break For_loop_START
                	}
                }
                
                MasterST = MASTER_2_ST			
			case MASTER_2_ST:
				
			case MASTER_3_ST:
				
			case MASTER_4_ST:
				
			case MASTER_5_ST:
				
			case MASTER_6_ST:
				
			default:
				fmt.Println("DS_ SOMETHING WENT TERRIBLE WRONG --------MASTER STATE INVALID")
				return STANDBY_ST
		}	
	}	
}

func LocalIPgreater (LocalIP string, OtherIP string) bool {
	var LocalIPnum int
	var RemoteIPnum int
	var err error

    LocalIPtmp := strings.SplitN(LocalIP,".",4)
    LocalIPnum,err = strconv.Atoi(LocalIPtmp[3])
    check(err)
    
    RemoteIPtmp := strings.SplitN(OtherIP,".",4)
    RemoteIPnum,err = strconv.Atoi(RemoteIPtmp[3])
    check(err)

	return (LocalIPnum>RemoteIPnum)
}


func check(err error){
    if err != nil{
        fmt.Fprintf(os.Stderr,time.Now().Format(LAYOUT_TIME))
        fmt.Fprintf(os.Stderr,"NET_  Error: %s\n",err.Error())
    }
}
