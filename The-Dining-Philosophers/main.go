package main

import (
	"fmt"
	"strconv"
)

/*
By making two of the philosophers right handed, the solution prevents deadlocks by being asymmetric.
Asymmetry prevents a "circular-wait" condition (See https://diningphilosophers.eu/hierarchy_asymmetric/) which is a deadlock condition.
The solution does however, not take starvation into consideration.
*/
func main() {
	n := 5

	chans := make([]chan bool, n)

	for i := range chans {
		chans[i] = make(chan bool, 1)
		chans[i] <- true
	}

	for i := range chans {
		if (i+1)%2 == 1 {
			go philosopherLeftHanded("Philosopher-"+strconv.Itoa(i+1), chans[i], chans[(i+1)%n])
		} else {
			go philosopherRightHanded("Philosopher-"+strconv.Itoa(i+1), chans[i], chans[(i+1)%n])
		}
	}

	select {}
}

func philosopherRightHanded(name string, rightFork chan bool, leftFork chan bool) {
	eatingCounter := 0
	for {
		if <-rightFork && <-leftFork {
			eatingCounter++
			fmt.Printf("\n%v is eating and has now eaten %d times.\n", name, eatingCounter)
			rightFork <- true
			leftFork <- true
			fmt.Printf("\n%v thinks.\n", name)
		}
	}
}

func philosopherLeftHanded(name string, rightFork chan bool, leftFork chan bool) {
	eatingCounter := 0
	for {
		if <-leftFork && <-rightFork {
			eatingCounter++
			fmt.Printf("\n%v is eating and has now eaten %d times.\n", name, eatingCounter)
			leftFork <- true
			rightFork <- true
			fmt.Printf("\n%v thinks.\n", name)
		}
	}
}
