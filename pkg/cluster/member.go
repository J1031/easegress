package cluster

import (
	"math/rand"
	"net"
	"time"

	"github.com/hexdecteam/easegateway/pkg/common"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

////

type Member struct {
	NodeName string
	NodeTags map[string]string
	Address  net.IP
	Port     uint16

	Status MemberStatus

	clusterProtocolMin, clusterProtocolMax, clusterProtocolCurrent          uint8
	memberListProtocolMin, memberListProtocolMax, memberListProtocolCurrent uint8
}

////

type memberStatus struct {
	Member
	lastMessageTime logicalTime
	goneTime        time.Time // local wall clock time, for cleanup
}

type memberStatusBook struct { // locking from outside
	members []*memberStatus
	timeout time.Duration
}

func createMemberStatusBook(timeout time.Duration) *memberStatusBook {
	return &memberStatusBook{
		timeout: timeout,
	}
}

func (msb *memberStatusBook) Count() int {
	return len(msb.members)
}

func (msb *memberStatusBook) add(member *memberStatus) {
	msb.members = append(msb.members, member)
}

func (msb *memberStatusBook) randGet() *memberStatus {
	return msb.members[rand.Int31n(int32(len(msb.members)))]
}

func (msb *memberStatusBook) remove(memberName string) int {
	var members []*memberStatus
	removed := 0

	for _, ms := range msb.members {
		if ms.NodeName == memberName {
			removed++
		} else {
			members = append(members, ms)
		}
	}

	msb.members = members

	return removed
}

func (msb *memberStatusBook) cleanup(now time.Time) []*memberStatus {
	var keepMembers, removedMembers []*memberStatus

	for _, ms := range msb.members {
		if now.Sub(ms.goneTime) <= msb.timeout {
			keepMembers = append(keepMembers, ms)
		} else {
			removedMembers = append(removedMembers, ms)
		}
	}

	msb.members = keepMembers

	return removedMembers
}

func (msb *memberStatusBook) names() []string {
	var ret []string

	for _, ms := range msb.members {
		ret = append(ret, ms.NodeName)
	}

	return ret
}

////

type memberOperation struct {
	msgType     messageType
	messageTime logicalTime
	receiveTime time.Time // local wall clock time, for cleanup
}

type memberOperationBook struct { // locking from outside
	operations map[string]*memberOperation
	timeout    time.Duration
}

func createMemberOperationBook(timeout time.Duration) *memberOperationBook {
	return &memberOperationBook{
		operations: make(map[string]*memberOperation),
		timeout:    timeout,
	}
}

func (mob *memberOperationBook) save(msgType messageType, nodeName string, msgTime logicalTime) bool {
	operation, ok := mob.operations[nodeName]
	if !ok || msgTime > operation.messageTime {
		mob.operations[nodeName] = &memberOperation{
			msgType:     msgType,
			messageTime: msgTime,
			receiveTime: common.Now(),
		}
		return true
	}

	return false
}

func (mob *memberOperationBook) get(nodeName string, msgType messageType) (bool, logicalTime) {
	operation, ok := mob.operations[nodeName]
	if !ok || operation.msgType != msgType {
		return false, zeroLogicalTime
	}

	return true, operation.messageTime
}

func (mob *memberOperationBook) cleanup(now time.Time) {
	for nodeName, operation := range mob.operations {
		if now.Sub(operation.receiveTime) > mob.timeout {
			delete(mob.operations, nodeName)
		}
	}
}