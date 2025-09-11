package main

import (
	"fmt"
)

func main() {
	fork1 := make(chan bool, 1)
	fork2 := make(chan bool, 1)
	fork3 := make(chan bool, 1)
	fork4 := make(chan bool, 1)
	fork5 := make(chan bool, 1)

	go philosopherLeftHanded("Philosopher-1", fork5, fork1)
	go philosopherRightHanded("Philosopher-2", fork1, fork2)
	go philosopherLeftHanded("Philosopher-3", fork2, fork3)
	go philosopherRightHanded("Philosopher-4", fork3, fork4)
	go philosopherRightHanded("Philosopher-5", fork4, fork5)

	fork1 <- true
	fork2 <- true
	fork3 <- true
	fork4 <- true
	fork5 <- true
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
