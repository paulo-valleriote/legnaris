package template

type Queue[T any] struct {
	items []T
}

func NewQueue[T any]() Queue[T] {
	return Queue[T]{
		items: make([]T, 0),
	}
}

func (q *Queue[T]) Pop() T {
	item := q.items[0]
	var zero T
	q.items[0] = zero
	q.items = q.items[1:]
	return item
}

func (q *Queue[T]) Append(data T) {
	q.items = append(q.items, data)
}

func (q *Queue[T]) Size() int {
	return len(q.items)
}
