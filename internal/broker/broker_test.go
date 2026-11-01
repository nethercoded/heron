package broker

import "testing"

func TestBroker(t *testing.T) {
	b := New[int](1)
	first, cancelFirst := b.Subscribe()
	second, cancelSecond := b.Subscribe()

	b.Publish(1)
	b.Publish(2)

	if v := <-first; v != 1 {
		t.Errorf("first got %d", v)
	}
	if v := <-second; v != 1 {
		t.Errorf("second got %d", v)
	}

	cancelFirst()
	cancelFirst()
	if _, ok := <-first; ok {
		t.Error("channel not closed after cancel")
	}
	if n := b.Subscribers(); n != 1 {
		t.Errorf("subscribers = %d", n)
	}

	b.Publish(3)
	if v := <-second; v != 3 {
		t.Errorf("second got %d", v)
	}
	cancelSecond()
}
