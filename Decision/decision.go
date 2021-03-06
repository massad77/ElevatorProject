package decision

import(
    "fmt"
    "time"
    //"os"
    "container/list"   //For using lists
	".././Redundancy"
    ".././Network"
    ".././Server"
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

func DecisionManager(ChanToServer chan<- server.ServerMsg, ChanFromNetwork <-chan network.Message, ChanToNetwork chan<- network.Message, ChanToRedun chan<- redundancy.TableReqMessage ){

	var MainState int
	MainState = STANDBY_ST

	//-----------------NETWORK
	var NetworkMsg network.Message
	var MsgToNetwork network.Message

	//---------------SERVER
    // Channel for receiving data from the server
	ChanToServer_Decision_ElementQueue := make(chan server.ElementQueue)

	var dummyElement server.ElementQueue
	var MsgToServer server.ServerMsg

	fmt.Println("DS_ Decision module started!")
	
	//Wait 2 seconds until all the variables get read 
	time.Sleep(4000*time.Millisecond)

	//Tick for check if the ReqQueue is empty
	timeout := time.Tick(200*time.Millisecond)
	for{
		switch(MainState){
			case STANDBY_ST:
				if(DEBUG){fmt.Println("DS_ STANDBY STATE BEGIN")}
				//In standby state, only two options
				select{
				    case NetworkMsg =<- ChanFromNetwork:
				    //Go to slave mode only if you receive a START message
				    if(NetworkMsg.MsgType == network.START){
    				    if(DEBUG){fmt.Println("DS_ GET INTO SLAVE", time.Now())}
				    	MainState = SLAVE_ST
				    	//Wait for the MASTER to finish its 10 millisecond wait time in case any other MASTER exist
			    	    time.Sleep(50*time.Millisecond)
	                    //Send ACK to master
	                    MsgToNetwork = 	network.Message{}
	                    MsgToNetwork.IDsender = "dummy"  //Filled out by network module
	                    MsgToNetwork.IDreceiver = NetworkMsg.IDsender
	                    MsgToNetwork.MsgType = network.ACK
	                    //MsgToNetwork.SizeGotoQueue = 0
	                    //MsgToNetwork.SizeMoveQueue = 0
	                    //MsgToNetwork.GotoQueue = buf
	                    //MsgToNetwork.MoveQueue = buf
	                    //MsgToNetwork.ActualPos = 0
	                    ChanToNetwork <- MsgToNetwork
                        if(DEBUG){fmt.Println("DS_ SLAVE first ACK sent", time.Now())}
				    }

				    case <- timeout:
				    	//Send message to server to know if the Req is empty
				    	//Extract first element from request queue
						MsgToServer.Cmd = server.CMD_READ_FIRST
						MsgToServer.QueueID = server.ID_REQQUEUE
						MsgToServer.ChanVal = ChanToServer_Decision_ElementQueue
						MsgToServer.ChanQueue = nil

						ChanToServer <- MsgToServer
						dummyElement =<- ChanToServer_Decision_ElementQueue

						if (dummyElement.Floor != -1){
						    if(DEBUG){fmt.Println("DS_ GET INTO MASTER", time.Now())}
							MainState = MASTER_ST
						}
		    }

			case MASTER_ST:
				if(DEBUG){fmt.Println("DS_ MASTER STATE BEGIN", time.Now())}
				MainState = master(ChanToServer, ChanFromNetwork, ChanToNetwork, ChanToRedun)

			case SLAVE_ST:
				if(DEBUG){fmt.Println("DS_ SLAVE STATE BEGIN", time.Now())}
				MainState = slave(ChanToServer, ChanFromNetwork, ChanToNetwork)
				//Sleep for some time in order to give the master a chance to read from the server
				//the request queue
				time.Sleep(50*time.Millisecond)

			default:
				fmt.Println("DS_ SOMETHING WENT TERRIBLE WRONG --------MAIN STATE INVALID")
				MainState = STANDBY_ST
		}
    }
}

func master (ChanToServer chan<- server.ServerMsg, ChanFromNetwork <-chan network.Message, ChanToNetwork chan<- network.Message, ChanToRedun chan<- redundancy.TableReqMessage) int {

//---------------MASTER
	var MasterST int
	MasterST = MASTER_1_ST
	//TABLE for record who has answer
	var ACKcounter int
	var ParticipantElement *list.Element
    var BackElement *list.Element
    var FrontElement  *list.Element
    var ActualPosElement int
    var ElementInAnyQueue bool
    var TargetParticipant string
    var SameDirectionParticipants *list.List
    SameDirectionParticipants = list.New()
    var FloorDifference int
    var LastFloorDifference int
    var CmdSuccessful bool

//-----------------NETWORK
	var MsgToNetwork network.Message
	var MsgFromNetwork network.Message


//---------------SERVER
    // Channel for receiving data from the server
	ChanToServer_Decision_ElementQueue := make(chan server.ElementQueue)

	var dummyElement server.ElementQueue
	var MsgToServer server.ServerMsg
	var dummyParticipant redundancy.Participant

//--------------REDUNDANCY
    ChanToRedun_Dec_Queue := make(chan *list.List)
    var TableReq redundancy.TableReqMessage
    TableReq.ChanQueue = ChanToRedun_Dec_Queue

	var TempList *list.List
    var ParticipantsList *list.List
    ParticipantsList = list.New()

	for{
		switch(MasterST){
			case MASTER_1_ST:
				if(DEBUG){fmt.Println("DS_ MASTER_1_ST")}
				//Request the participants table
				ChanToRedun <- TableReq
				TempList =<- ChanToRedun_Dec_Queue
				ParticipantsList.Init()
				ParticipantsList.PushBackList(TempList)

				//Reset message that will be sent to other elevators
				MsgToNetwork = 	network.Message{}
				//MsgToNetwork.IDsender = "dummy"  //Filled out by network module
				//MsgToNetwork.IDreceiver = "Broadcast"
				MsgToNetwork.MsgType = network.START
				//MsgToNetwork.SizeGotoQueue = 0
				//MsgToNetwork.SizeMoveQueue = 0
				//MsgToNetwork.GotoQueue = buf
				//MsgToNetwork.MoveQueue = buf
				//MsgToNetwork.ActualPos = 0


				//Go through participants table and send a START message throught the network
                for e := ParticipantsList.Front(); e != nil; e = e.Next(){
                	MsgToNetwork.IDreceiver = e.Value.(redundancy.Participant).IPsender
                	//Do not send a message to yourself
                	if(MsgToNetwork.IDreceiver != "Local"){
                		ChanToNetwork <- MsgToNetwork
                	}else{
                	    //Set your own AckResponse flag to true
                        //As you will never send to yourself niether a cmd START nor a cmd ACK
                        //Pointers in GO are stupid!! When you write to element you have to cast to a pointer
                        //e.Value.(*redundancy.Participant).AckResponse = true
                        dummyParticipant = e.Value.(redundancy.Participant)
                        dummyParticipant.AckResponse = true
                        e.Value = dummyParticipant
                	}
                }

				//Check if you have received a START msg from other elevator
               	timeout := time.Tick(10*time.Millisecond)
               	//Normal ping takes 0.3ms
For_loop_START: for{
                	select{
                		//Wait until you receive a smaller IP if any other elevator become a MASTER
                		case MsgFromNetwork =<- ChanFromNetwork:
                			if(MsgFromNetwork.MsgType == network.START){
                				if(!LocalIPgreater(network.LocalIP,MsgFromNetwork.IDsender)){
                					//If your are not the master with the lowest ID then
                					//go to SLAVE_ST

							    	//Wait for the MASTER to finish its 10 millisecond wait time in case any other MASTER exist
                					time.Sleep(50*time.Millisecond)
                					//Reset message that will be sent to other elevators
									MsgToNetwork = 	network.Message{}
                					MsgToNetwork.IDsender = "dummy"  //Filled out by network module
									MsgToNetwork.IDreceiver = MsgFromNetwork.IDsender
									MsgToNetwork.MsgType = network.ACK
									//MsgToNetwork.SizeGotoQueue = 0
									//MsgToNetwork.SizeMoveQueue = 0
									//MsgToNetwork.GotoQueue = buf
									//MsgToNetwork.MoveQueue = buf
									//MsgToNetwork.ActualPos = 0
									ChanToNetwork <- MsgToNetwork
                					return SLAVE_ST
                				}
							}
						case <-timeout:
							break For_loop_START
                	}
                }

                MasterST = MASTER_2_ST
			case MASTER_2_ST:
			if(DEBUG){fmt.Println("DS_ MASTER_2_ST")}
			timeout_response := time.Tick(500*time.Millisecond)
			ACKcounter = 1
For_loop_RESPONSE_ACK:
				for{
                	select{
                		//Wait until you have received all ACK from slaves
                		case MsgFromNetwork =<- ChanFromNetwork:
                			if(MsgFromNetwork.MsgType == network.ACK){
								ParticipantElement = redundancy.PartOfParticipantList(ParticipantsList, MsgFromNetwork.IDsender)
								if (ParticipantElement != nil){
									dummyParticipant = ParticipantElement.Value.(redundancy.Participant)
						            dummyParticipant.AckResponse = true
						            ParticipantElement.Value = dummyParticipant
									ACKcounter++
								}
							}
						case <-timeout_response:
							break For_loop_RESPONSE_ACK
                	}
                }
				//If you have received all the ACK then go on, otherwise erase the element from the Participants table
                if(ACKcounter == ParticipantsList.Len()){
                	if(DEBUG){fmt.Println("DS_ MASTER 2->3")}
                	MasterST = MASTER_3_ST
                }else{
	                if(DEBUG){fmt.Println("DS_ MASTER 2->4")}
	                MasterST = MASTER_4_ST
                }
			case MASTER_3_ST:
				if(DEBUG){fmt.Println("DS_ MASTER_3_ST")}
                TargetParticipant = ""
                ElementInAnyQueue = false
                FloorDifference = server.FLOORS
                SameDirectionParticipants.Init()

				// Read first element from request queue
				MsgToServer.Cmd = server.CMD_READ_FIRST
				MsgToServer.QueueID = server.ID_REQQUEUE
				MsgToServer.ChanVal = ChanToServer_Decision_ElementQueue
				MsgToServer.ChanQueue = nil

				ChanToServer <- MsgToServer
				dummyElement =<- ChanToServer_Decision_ElementQueue

                // if the request came from inside your elevator put it in your GotoQueue
				if(dummyElement.Direction == server.NONE) {
					for e := ParticipantsList.Front(); e != nil; e = e.Next(){
		                if(e.Value.(redundancy.Participant).IPsender == "Local"){
							if(fitInQueueLocal(e.Value.(redundancy.Participant).GotoQueue,e.Value.(redundancy.Participant).ActualPos,dummyElement) == false){
                                e.Value.(redundancy.Participant).GotoQueue.PushBack(dummyElement)
                            }
                            TargetParticipant = "Local"
                            if(DEBUG){
                            	fmt.Println("DS_ Request put in own queue")
                                redundancy.PrintList(e.Value.(redundancy.Participant).GotoQueue)
                            }
						}
		            }
                } else {
                    // check if element is already in any queue
                	for e := ParticipantsList.Front(); e != nil; e = e.Next(){
                        for f := e.Value.(redundancy.Participant).GotoQueue.Front(); f != nil; f = f.Next(){
                            if(f.Value.(server.ElementQueue) == dummyElement){
                                ElementInAnyQueue = true
                                if(DEBUG){fmt.Println("DS_ MASTER_3_ST Element already in any queue")}
                                goto DecisionDone
                            }
                        }
		            }

                    // Search for free elevator
                    for e := ParticipantsList.Front(); e != nil; e = e.Next(){
	                    if(e.Value.(redundancy.Participant).GotoQueue.Len() == 0){
						    e.Value.(redundancy.Participant).GotoQueue.PushBack(dummyElement)
                            TargetParticipant = e.Value.(redundancy.Participant).IPsender
                            if(DEBUG){fmt.Println("DS_ MASTER_3_ST There was a free elevator and it got the request element")}
                            goto DecisionDone
					    }
	                }

                    //Get all participants which last element is going in the same direction as you
                    for e := ParticipantsList.Front(); e != nil; e = e.Next(){
                        BackElement = e.Value.(redundancy.Participant).GotoQueue.Back()

	                    if(BackElement.Value.(server.ElementQueue).Direction == dummyElement.Direction){
                            SameDirectionParticipants.PushBack(e.Value.(redundancy.Participant))
					    }
	                }

                    // check if request fits in a participant which last element has the same direction
                    if(SameDirectionParticipants != nil) {
					    for e := SameDirectionParticipants.Front(); e != nil; e = e.Next(){
                            if(dummyElement.Direction == server.UP){
                                if(e.Value.(redundancy.Participant).GotoQueue.Back().Value.(server.ElementQueue).Floor < dummyElement.Floor){
                                    TargetParticipant = e.Value.(redundancy.Participant).IPsender
                                    e.Value.(redundancy.Participant).GotoQueue.PushBack(dummyElement)
                                    if(DEBUG){fmt.Println("DS_ MASTER_3_ST Element inserted in a participant with the last direction UP")}
                                    goto DecisionDone
                                 }
                            }else{
                                if(e.Value.(redundancy.Participant).GotoQueue.Back().Value.(server.ElementQueue).Floor > dummyElement.Floor){
                                    TargetParticipant = e.Value.(redundancy.Participant).IPsender
                                    e.Value.(redundancy.Participant).GotoQueue.PushBack(dummyElement)
                                    if(DEBUG){fmt.Println("DS_ MASTER_3_ST Element inserted in a participant with the last direction DOWN")}
                                    goto DecisionDone
                                 }
                            }
		                }
                    }

                    //Try to fit the new request between the actual position and the first element of the queue
                    //Get all participants which actual movement is the same as your request
                    for e := ParticipantsList.Front(); e != nil; e = e.Next(){
                        FrontElement = e.Value.(redundancy.Participant).GotoQueue.Back()
                        ActualPosElement = e.Value.(redundancy.Participant).ActualPos

                        FloorDifference = ActualPosElement - FrontElement.Value.(server.ElementQueue).Floor

                        if(FloorDifference < 0){
                            FloorDifference = server.UP
                        }else{
                            FloorDifference = server.DOWN
                        }

	                    if(FloorDifference == dummyElement.Direction){
                            SameDirectionParticipants.PushBack(e.Value.(redundancy.Participant))
					    }
	                }

                    // check if request fits between the actual position and the first element
                    if(SameDirectionParticipants != nil) {
					    for e := SameDirectionParticipants.Front(); e != nil; e = e.Next(){
                            if(dummyElement.Direction == server.UP){
                                if(e.Value.(redundancy.Participant).GotoQueue.Front().Value.(server.ElementQueue).Floor > dummyElement.Floor && e.Value.(redundancy.Participant).ActualPos < dummyElement.Floor){
                                    TargetParticipant = e.Value.(redundancy.Participant).IPsender
                                    e.Value.(redundancy.Participant).GotoQueue.InsertBefore(dummyElement, e.Value.(redundancy.Participant).GotoQueue.Front())
                                    if(DEBUG){fmt.Println("DS_ MASTER_3_ST Element inserted in a participant at the begin UP")}
                                    goto DecisionDone
                                 }
                            }else{
                                if(e.Value.(redundancy.Participant).GotoQueue.Back().Value.(server.ElementQueue).Floor < dummyElement.Floor && e.Value.(redundancy.Participant).ActualPos > dummyElement.Floor){
                                    TargetParticipant = e.Value.(redundancy.Participant).IPsender
                                    e.Value.(redundancy.Participant).GotoQueue.InsertBefore(dummyElement, e.Value.(redundancy.Participant).GotoQueue.Front())
                                    if(DEBUG){fmt.Println("DS_ MASTER_3_ST Element inserted in a participant at the begin DOWN")}
                                    goto DecisionDone
                                 }
                            }
		                }
                    }

                    // new request does not fit at the end of any queues
                    // Get Queue with closest end position
                    LastFloorDifference = server.FLOORS
                    for e := ParticipantsList.Front(); e != nil; e = e.Next(){
                        BackElement = e.Value.(redundancy.Participant).GotoQueue.Back()
                        FloorDifference = BackElement.Value.(server.ElementQueue).Floor - dummyElement.Floor
                        if FloorDifference < 0 {
                            FloorDifference *= -1
                        }
                        if( FloorDifference < LastFloorDifference){
                            LastFloorDifference = FloorDifference
                            TargetParticipant = e.Value.(redundancy.Participant).IPsender
                        }
		            }
		            if(DEBUG){fmt.Println("DS_ Get Participant from table", TargetParticipant)}
                    ParticipantElement = redundancy.PartOfParticipantList(ParticipantsList,TargetParticipant)
                    // and add the new request to its queue
   		            if(DEBUG){fmt.Println("DS_ Insert element in gotoqueue")}
                    ParticipantElement.Value.(redundancy.Participant).GotoQueue.PushBack(dummyElement)
                    if(DEBUG){fmt.Println("DS_ Insert element in gotoqueue done")}
                }


        DecisionDone:
                // Get participant that gets the new Gotoqueue
                ParticipantElement = redundancy.PartOfParticipantList(ParticipantsList,TargetParticipant)

                // Send new Gotoqueue over network if the request was not in any queue already and was not put in you own queue
                if(ElementInAnyQueue == false && TargetParticipant != "Local"){
                    if(DEBUG){ fmt.Println(TargetParticipant) }

            	    //Set all AckResponse flag to true except from the one which will receive the CMD
                    for e := ParticipantsList.Front(); e != nil; e = e.Next(){
                        if e.Value.(redundancy.Participant).IPsender != TargetParticipant {
                            dummyParticipant = e.Value.(redundancy.Participant)
                            dummyParticipant.AckResponse = true
                            e.Value = dummyParticipant
                        } else {
                            dummyParticipant = e.Value.(redundancy.Participant)
                            dummyParticipant.AckResponse = false
                            e.Value = dummyParticipant
                        }
                    }

					//Send the new Gotoqueue
					if(DEBUG){fmt.Println("DS_ MASTER_3_ST Send cmd selected slave ", TargetParticipant)}
					MsgToNetwork = 	network.Message{}
					MsgToNetwork.IDsender = "dummy"  //Filled out by network module
					MsgToNetwork.IDreceiver = TargetParticipant
					MsgToNetwork.MsgType = network.CMD
					MsgToNetwork.SizeGotoQueue = ParticipantElement.Value.(redundancy.Participant).GotoQueue.Len()
					//MsgToNetwork.SizeMoveQueue = 0
					MsgToNetwork.GotoQueue = redundancy.ListToArray(ParticipantElement.Value.(redundancy.Participant).GotoQueue)
					//MsgToNetwork.MoveQueue = buf
					//MsgToNetwork.ActualPos = 0
					ChanToNetwork <- MsgToNetwork

                    // Wait for an acknowledge from the one which got the queue
                    timeout_response := time.Tick(100*time.Millisecond)
    For_loop_RESPONSE_CMD_ACK:
			        for{
                    	select{
                    		case MsgFromNetwork =<- ChanFromNetwork:
                    			if(MsgFromNetwork.MsgType == network.ACK){
							        ParticipantElement = redundancy.PartOfParticipantList(ParticipantsList, MsgFromNetwork.IDsender)
							        if (ParticipantElement != nil && ParticipantElement.Value.(redundancy.Participant).IPsender == TargetParticipant){
								        dummyParticipant = ParticipantElement.Value.(redundancy.Participant)
					                    dummyParticipant.AckResponse = true
					                    ParticipantElement.Value = dummyParticipant
                                        CmdSuccessful = true
                                        break For_loop_RESPONSE_CMD_ACK
							        }
						        }
					        case <-timeout_response:
                                CmdSuccessful = false
                                MasterST = MASTER_4_ST
						        break For_loop_RESPONSE_CMD_ACK
                    	}
                    }
                }

                // If either one has successfully received the new queue or it was already in a queue or you took it
                if(CmdSuccessful == true || ElementInAnyQueue == true || TargetParticipant == "Local"){
				    // Extract first element from request queue
				    MsgToServer.Cmd = server.CMD_EXTRACT
				    MsgToServer.QueueID = server.ID_REQQUEUE
				    MsgToServer.ChanVal = ChanToServer_Decision_ElementQueue
				    MsgToServer.ChanQueue = nil

				    ChanToServer <- MsgToServer
				    dummyElement =<- ChanToServer_Decision_ElementQueue

                    if(TargetParticipant == "Local") {
    					// Send new GotoQueue to server
						if(DEBUG){fmt.Println("DS_ MASTER_3_ST Send the request element to your gotoqueue")}
						MsgToServer.Cmd = server.CMD_REPLACE_ALL
						MsgToServer.QueueID = server.ID_GOTOQUEUE
						MsgToServer.NewQueue = ParticipantElement.Value.(redundancy.Participant).GotoQueue
						MsgToServer.ChanVal = nil
						MsgToServer.ChanQueue = nil

						ChanToServer <- MsgToServer
                    }
                    MasterST = MASTER_5_ST
                }

			case MASTER_4_ST:
			    if(DEBUG){fmt.Println("DS_ MASTER_4_ST")}
			    for e := ParticipantsList.Front(); e != nil; e = e.Next(){
                    if (e.Value.(redundancy.Participant).AckResponse == false){
					    ParticipantsList.Remove(e)
					}
                }
                MasterST = MASTER_3_ST
			case MASTER_5_ST:
				if(DEBUG){fmt.Println("DS_ MASTER_5_ST")}
				//First send LAST_ACK

				//Reset message that will be sent to the slaves
				MsgToNetwork = 	network.Message{}
				//MsgToNetwork.IDsender = "dummy"  //Filled out by network module
				//MsgToNetwork.IDreceiver = "Broadcast"
				MsgToNetwork.MsgType = network.LAST_ACK
				//MsgToNetwork.SizeGotoQueue = 0
				//MsgToNetwork.SizeMoveQueue = 0
				//MsgToNetwork.GotoQueue = buf
				//MsgToNetwork.MoveQueue = buf
				//MsgToNetwork.ActualPos = 0

				//Go through participants table and send a LAST_ACK message throught the network
                for e := ParticipantsList.Front(); e != nil; e = e.Next(){
                	MsgToNetwork.IDreceiver = e.Value.(redundancy.Participant).IPsender
                	//Do not send a message to yourself
                	if(MsgToNetwork.IDreceiver != "Local"){
                		ChanToNetwork <- MsgToNetwork
                	}
                }

				//Wait to give redundancy time to update the participants table
				time.Sleep(150*time.Millisecond)
				//Send message to server to know if the Req is empty
				// Read first element from request queue
				MsgToServer.Cmd = server.CMD_READ_FIRST
				MsgToServer.QueueID = server.ID_REQQUEUE
				MsgToServer.ChanVal = ChanToServer_Decision_ElementQueue
				MsgToServer.ChanQueue = nil

				ChanToServer <- MsgToServer
				dummyElement =<- ChanToServer_Decision_ElementQueue

				if (dummyElement.Floor != -1){
					MasterST = MASTER_1_ST
				}else{
				    return STANDBY_ST
				}
			default:
				fmt.Println("DS_ SOMETHING WENT TERRIBLE WRONG --------MASTER STATE INVALID")
				return STANDBY_ST
		}
	}
}


func slave (ChanToServer chan<- server.ServerMsg, ChanFromNetwork <-chan network.Message, ChanToNetwork chan<- network.Message) int {

//-----------------NETWORK
	var MsgToNetwork network.Message
	var MsgFromNetwork network.Message

//---------------SERVER
	var MsgToServer server.ServerMsg

	var TempList *list.List
    var GotoQueue *list.List
    GotoQueue = list.New()

    //Maximum time for the Master to get all ACK, make decision, send CMD and send LAST_ACK
    // Maybe increase time because master sleeps at beginning to get latest participant table
    timeout_req_from_master := time.Tick(1000*time.Millisecond)

    for{
    	select{
    		case MsgFromNetwork =<- ChanFromNetwork:
    			switch (MsgFromNetwork.MsgType){
    				case network.START:
    					if(DEBUG){fmt.Println("DS_ SLAVE Start", time.Now())}
				    	//Wait for the MASTER to finish its 10 millisecond wait time in case any other MASTER exist
                        time.Sleep(50*time.Millisecond)
    					//Reset message that will be sent to the master
						MsgToNetwork = 	network.Message{}
    					MsgToNetwork.IDsender = "dummy"  //Filled out by network module
						MsgToNetwork.IDreceiver = MsgFromNetwork.IDsender
						MsgToNetwork.MsgType = network.ACK
						//MsgToNetwork.SizeGotoQueue = 0
						//MsgToNetwork.SizeMoveQueue = 0
						//MsgToNetwork.GotoQueue = buf
						//MsgToNetwork.MoveQueue = buf
						//MsgToNetwork.ActualPos = 0
						ChanToNetwork <- MsgToNetwork
    				case network.CMD:
	    				if(DEBUG){fmt.Println("DS_ SLAVE Cmd", time.Now())}
    					TempList = redundancy.ArrayToList(MsgFromNetwork.GotoQueue, MsgFromNetwork.SizeGotoQueue)
    					GotoQueue.Init()
    					GotoQueue.PushBackList(TempList)

    					// Send new GotoQueue to server
						MsgToServer.Cmd = server.CMD_REPLACE_ALL
						MsgToServer.QueueID = server.ID_GOTOQUEUE
						MsgToServer.NewQueue = GotoQueue
						MsgToServer.ChanVal = nil
						MsgToServer.ChanQueue = nil

						ChanToServer <- MsgToServer

    					//Send ACK to master that we have done the changes
    					MsgToNetwork = 	network.Message{}
    					MsgToNetwork.IDsender = "dummy"  //Filled out by network module
						MsgToNetwork.IDreceiver = MsgFromNetwork.IDsender
						MsgToNetwork.MsgType = network.ACK
						//MsgToNetwork.SizeGotoQueue = 0
						//MsgToNetwork.SizeMoveQueue = 0
						//MsgToNetwork.GotoQueue = buf
						//MsgToNetwork.MoveQueue = buf
						//MsgToNetwork.ActualPos = 0
						ChanToNetwork <- MsgToNetwork

    				case network.LAST_ACK:
	    				if(DEBUG){fmt.Println("DS_ SLAVE LastAck", time.Now())}
    					return STANDBY_ST
    				default:
    					if(DEBUG){ fmt.Println("DC_ Being SLAVE and received a ACK Message... Something Wrong", time.Now()) }
    			}
			case <-timeout_req_from_master:
				//If the timeout expired, the something is wrong with the master as it has taken more than 1 second
				if(DEBUG){fmt.Println("DS_ SLAVE Timeout", time.Now())}
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
    network.Check(err)

    RemoteIPtmp := strings.SplitN(OtherIP,".",4)
    RemoteIPnum,err = strconv.Atoi(RemoteIPtmp[3])
    network.Check(err)

	return (LocalIPnum>RemoteIPnum)
}

func fitInQueueLocal(List *list.List,ActualPos int,NewElement server.ElementQueue) bool {
	var ActualDirection int

    // check if the queue is empty, if so then just add the element
    if List.Front() == nil {
        List.PushBack(NewElement)
        if(DEBUG){ fmt.Println("Fit local: only element") }
        return true
    }

	// check if Element already in list
	for e := List.Front(); e != nil; e = e.Next(){
        if e.Value.(server.ElementQueue).Floor == NewElement.Floor {
            if (e.Value.(server.ElementQueue).Direction != server.NONE) {
            	List.InsertAfter(NewElement,e)
	            if(DEBUG){ fmt.Println("Fit local: already in list, element inserted after the one with the same floor diff direction") }
            }
            return true
        }
    }

    // check direction you are going
    ActualDirection = List.Front().Value.(server.ElementQueue).Floor - ActualPos

    // we want to go up
    if(ActualDirection >= 0) {
        if ActualPos < NewElement.Floor {
           for e := List.Front(); e != nil; e = e.Next(){
                if e.Value.(server.ElementQueue).Floor > NewElement.Floor {
                    List.InsertBefore(NewElement,e)
                    if(DEBUG){ fmt.Println("Fit local: up, inserted before") }
                    return true
                }

                if e.Next() == nil{
                    List.InsertAfter(NewElement,e)
                    return true
                }else if (e.Next().Value.(server.ElementQueue).Floor < e.Value.(server.ElementQueue).Floor) {
                    List.InsertAfter(NewElement,e)
                    if(DEBUG){ fmt.Println("Fit local: up, inserted after (turning point)") }
                    return true
                }
            }
        } else {
            for e := List.Back(); e != nil; e = e.Prev(){
                if e.Value.(server.ElementQueue).Floor > NewElement.Floor {
                    List.InsertAfter(NewElement,e)
                    if(DEBUG){ fmt.Println("Fit local: up, changed direction inserted after") }
                    return true
                }
            }
        }
    // else go down
    } else {
        if ActualPos > NewElement.Floor {
           for e := List.Front(); e != nil; e = e.Next(){
                if e.Value.(server.ElementQueue).Floor < NewElement.Floor {
                    List.InsertBefore(NewElement,e)
                    if(DEBUG){ fmt.Println("Fit local: down, inserted before") }
                    return true
                }
                if e.Next() == nil{
                    List.InsertAfter(NewElement,e)
                    return true
                } else if (e.Next().Value.(server.ElementQueue).Floor > e.Value.(server.ElementQueue).Floor) {
                    List.InsertAfter(NewElement,e)
                    if(DEBUG){ fmt.Println("Fit local: down, inserted after (turning point)") }
                    return true
                }
            }
        } else {
            for e := List.Back(); e != nil; e = e.Prev(){
                if e.Value.(server.ElementQueue).Floor < NewElement.Floor {
                    List.InsertAfter(NewElement,e)
                    if(DEBUG){ fmt.Println("Fit local: down, changed direction inserted after") }
                    return true
                }
            }
        }
    }
    return false

}

func fitInQueue(List *list.List,ActualPos int,NewElement server.ElementQueue) bool {
	var DirectionQueueUp int

	// check direction you want to go
    DirectionQueueUp = List.Front().Value.(server.ElementQueue).Floor - ActualPos

    // we want to go up
    if(DirectionQueueUp >= 0) {
        for e := List.Back(); e != nil; e = e.Prev(){
            if e.Value.(server.ElementQueue).Floor < NewElement.Floor {
                List.InsertAfter(NewElement,e)
                if(DEBUG){ fmt.Println("Fit: up, inserted after") }
                return true
            }
        }
        if ActualPos < NewElement.Floor {
            List.InsertBefore(NewElement,List.Front())
            if(DEBUG){ fmt.Println("Fit: up, inserted in beginning") }
            return true
        }
    // else go down
    } else {
        for e := List.Back(); e != nil; e = e.Prev(){
            if e.Value.(server.ElementQueue).Floor > NewElement.Floor {
                List.InsertAfter(NewElement,e)
                if(DEBUG){ fmt.Println("Fit: down, inserted after") }
                return true
            }
        }
        if ActualPos > NewElement.Floor {
            List.InsertBefore(NewElement,List.Front())
            if(DEBUG){ fmt.Println("Fit: down, inserted in beginning") }
            return true
        }

    }
    return false
}
