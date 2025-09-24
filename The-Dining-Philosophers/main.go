package main

import (
	"fmt"
)

/*
By making every even philosopher right handed, the solution prevents a deadlock by being asymmetric.
Asymmetry prevents a "circular-wait" condition (See https://diningphilosophers.eu/hierarchy_asymmetric/) which is a deadlock condition.
The solution does not treat philosophers fairly as the one closing the odd circut is heavily favoured, but there is no full starvation.
*/

type request struct {
	ask  chan struct{} // philosopher sends once to request
	done chan struct{} // fork replies once to grant; philosopher sends once to release
}

func fork(id int, reqCh <-chan request) {
	for req := range reqCh {
		// Wait for acquire
		<-req.ask
		// Grant fork
		req.done <- struct{}{}
		// Wait for release
		<-req.done
	}
}

func philosopher(id int, left, right chan request) {
	var eatCounter = 0
	// helper: acquire returns a release function that signals on the SAME done chan
	take := func(ch chan request) func() {
		ask := make(chan struct{})
		done := make(chan struct{})
		ch <- request{ask: ask, done: done}
		ask <- struct{}{} // request fork
		<-done            // granted
		return func() {
			done <- struct{}{} // release fork
		}
	}
	for {
		fmt.Printf("Philosopher %d thinking\n", id)

		// Break circular wait:
		first, second := left, right
		if id%2 == 0 {
			first, second = right, left
		}

		releaseFirst := take(first)
		releaseSecond := take(second)
		eatCounter++
		fmt.Printf("Philosopher %d eating and has eaten %d times\n ", id, eatCounter)

		releaseFirst()
		releaseSecond()
	}
}

func main() {

	const n = 3

	// One goroutine per fork
	forks := make([]chan request, n)
	for i := 0; i < n; i++ {
		forks[i] = make(chan request)
		go fork(i, forks[i])
	}

	// One goroutine per philosopher
	for i := 0; i < n; i++ {
		left := forks[i]
		right := forks[(i+1)%n]
		go philosopher(i, left, right)
	}
	select {}

}
