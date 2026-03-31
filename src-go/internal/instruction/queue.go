package instruction

type instructionQueueItem struct {
	request    Request
	definition Definition
	sequence   uint64
	index      int
}

type instructionPriorityQueue []*instructionQueueItem

func (q instructionPriorityQueue) Len() int {
	return len(q)
}

func (q instructionPriorityQueue) Less(i, j int) bool {
	if q[i].request.Priority == q[j].request.Priority {
		return q[i].sequence < q[j].sequence
	}
	return q[i].request.Priority > q[j].request.Priority
}

func (q instructionPriorityQueue) Swap(i, j int) {
	q[i], q[j] = q[j], q[i]
	q[i].index = i
	q[j].index = j
}

func (q *instructionPriorityQueue) Push(x any) {
	item := x.(*instructionQueueItem)
	item.index = len(*q)
	*q = append(*q, item)
}

func (q *instructionPriorityQueue) Pop() any {
	old := *q
	last := len(old) - 1
	item := old[last]
	old[last] = nil
	item.index = -1
	*q = old[:last]
	return item
}
