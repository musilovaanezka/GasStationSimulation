package main

import (
	"fmt"
	"math/rand"
	"os"
	"sync"
	"time"

	"gopkg.in/yaml.v2"
)

type StationType string

const (
    Gas     StationType = "gas"
    Diesel  StationType = "diesel"
    LPG     StationType = "lpg"
    Electric StationType = "electric"
)

type Car struct {
	ID				int 
	ArrivalTime		time.Time
	DepartureTime	time.Time
	QueueTime		time.Duration
	HandleTime		time.Duration
	RegisterTime	time.Duration
	FuelType		StationType 	
}

type Station struct {
	Type			StationType
	MinServeTime	time.Duration
	MaxServeTime	time.Duration
	TotalCars		int
	TotalTime		time.Duration
	AvgQueueTime	time.Duration
	MaxQueueTime	time.Duration
	ServeChan       chan struct{} 
	StationTimeStart 	time.Time
}

type Register struct {
	HandleTimeMin	time.Duration
	HandleTimeMax	time.Duration
	TotalCars		int
	TotalTime		time.Duration
	AvgQueueTime	time.Duration
	MaxQueueTime	time.Duration
	HandleChan      chan struct{}
	RegisterTimeStart 	time.Time
}

type Simulation struct {
	Cars		[]*Car			`yaml:"cars"`
	Stations      map[StationType]*Station `yaml:"stations"`
	CashRegisters []*Register     `yaml:"registers"`
}

type StationConfig struct {
    Count           int             `yaml:"count"`
    ServeTimeMin    time.Duration   `yaml:"serve_time_min"`
    ServeTimeMax    time.Duration   `yaml:"serve_time_max"`
}


type Config struct {
    Cars    struct {
        Count           int             `yaml:"count"`
        ArrivalTimeMin  time.Duration   `yaml:"arrival_time_min"`
        ArrivalTimeMax  time.Duration   `yaml:"arrival_time_max"`
    } `yaml:"cars"`
    Stations map[StationType]StationConfig `yaml:"stations"`
    Registers struct {
        Count           int             `yaml:"count"`
        HandleTimeMin   time.Duration   `yaml:"handle_time_min"`
        HandleTimeMax   time.Duration   `yaml:"handle_time_max"`
    } `yaml:"registers"`
}

func initiateSimulation(configFile string) (*Simulation, error) {
	yamlFile, err := os.ReadFile(configFile)
    if err != nil {
        panic(err)
    }

	var config Config
    err = yaml.Unmarshal(yamlFile, &config)
    if err != nil {
        panic(err)
    }

	simulation := &Simulation{
		Stations: 		make(map[StationType]*Station),
		CashRegisters:	make([]*Register, 0, config.Registers.Count),
	}

	for stationType, stationConfig := range config.Stations {
		for i := 0; i < stationConfig.Count; i++ {
			station := &Station{
				Type: 			stationType,
				MinServeTime:	stationConfig.ServeTimeMin,
				MaxServeTime:	stationConfig.ServeTimeMax,
				ServeChan:		make(chan struct{}, 1),
			}
			station.ServeChan <- struct{}{}
			simulation.Stations[stationType] = station
		}
	}

	for i := 0; i < config.Cars.Count; i++ {
		simulation.Cars = append(simulation.Cars, &Car{ID: i + 1})
	}

	for i := 0; i < config.Registers.Count; i++ {
		register := &Register{
			HandleTimeMin:	config.Registers.HandleTimeMin,
			HandleTimeMax:	config.Registers.HandleTimeMax,
			HandleChan:		make(chan struct{}, 1),
		}
		register.HandleChan <- struct{}{}
		simulation.CashRegisters = append(simulation.CashRegisters, register)
	}

	return simulation, nil
}

func getStationType(stations map[StationType]*Station) StationType {
	var types []StationType
	for stationType := range stations {
		types = append(types, StationType(stationType))
	}

	return types[rand.Intn(len(types))]
}

func findShortestQueue(stations map[StationType]*Station, stationType StationType) *Station {
	var shortestQueue *Station 
	for _, station := range stations {
		if station.Type == stationType {
			if shortestQueue == nil || station.TotalCars < shortestQueue.TotalCars {
				shortestQueue = station
			}
		}
	}
	return shortestQueue
}

func enterQueue(station *Station) {
	<-station.ServeChan                                // Wait until station is available
	defer func() { station.ServeChan <- struct{}{} }() // Signal that station is available
	station.TotalCars++
}

func getServeDuration(station *Station) time.Duration {
    serveDuration := time.Duration(rand.Intn(int(station.MaxServeTime-station.MinServeTime)) + int(station.MinServeTime))
	return serveDuration

}

func getHandleDuration(register *Register) time.Duration {
	handleDuration := time.Duration(rand.Intn(int(register.HandleTimeMax-register.HandleTimeMin)) + int(register.HandleTimeMin))
	return handleDuration
}

func serveCar(station *Station) time.Duration {
    serveTime := getServeDuration(station)
	time.Sleep(serveTime)
	station.TotalTime += serveTime
	return serveTime
}

func updateStationStats(station *Station) time.Duration {
	queueTime := time.Since(station.StationTimeStart)
	if queueTime > station.MaxQueueTime {
        station.MaxQueueTime = queueTime
    }
	return queueTime
}

func getRegister(registers []*Register) *Register {
	return registers[rand.Intn(len(registers))]
}

func handleCar(register *Register) time.Duration{
	<-register.HandleChan                                // Wait until cash register is available
	defer func() { register.HandleChan <- struct{}{} }() // Signal that cash register is available
	handleTime := getHandleDuration(register)
	time.Sleep(handleTime)
	register.TotalCars++
	register.TotalTime += handleTime
	if handleTime > register.MaxQueueTime {
        register.MaxQueueTime = handleTime
    }
	return handleTime
}

func calculateAvgQueueTime(stations map[StationType]*Station) {
	for _,station := range stations {
		if station.TotalCars == 0 {
			station.AvgQueueTime = 0
		}
		station.AvgQueueTime = station.TotalTime / time.Duration(station.TotalCars)
	}
}

func (simulation *Simulation) runSimulation(){
	var wg sync.WaitGroup
	finished := make(chan struct{})

	for _, car := range simulation.Cars {
		wg.Add(1)
		go func(c *Car) {
			defer wg.Done()

			car.ArrivalTime = time.Now()
			carStationType := getStationType(simulation.Stations)
			carStation := findShortestQueue(simulation.Stations, carStationType)
			carStation.StationTimeStart = time.Now()
			enterQueue(carStation)
			carRegisterTime := serveCar(carStation)
			queueTime := updateStationStats(carStation)

			register := getRegister(simulation.CashRegisters)
			handleTime := handleCar(register)

			car.DepartureTime = time.Now()
			car.HandleTime = handleTime
			car.QueueTime = queueTime
			car.RegisterTime = carRegisterTime
		}(car)
	}

	go func() {
		defer close(finished)
		wg.Wait()
	}()

	<-finished
}

func (simulation *Simulation) printResults() {
	fmt.Println("Stations:")
	calculateAvgQueueTime(simulation.Stations)
	for stationType, station := range simulation.Stations {
		fmt.Printf("	%s:\n", stationType)
		fmt.Printf("		total_cars: %d\n", station.TotalCars)
		fmt.Printf("		total_time: %s\n", station.TotalTime)
		fmt.Printf("		avg_queue_time: %s\n", station.AvgQueueTime)
		fmt.Printf("		max_queue_time: %s\n", station.MaxQueueTime)
	}

	totalCarsRegisters := 0
	totalTimeRegisters := time.Duration(0)
	maxQueueTimeRegisters := time.Duration(0)

	for _, register := range simulation.CashRegisters {
		totalCarsRegisters += register.TotalCars
		totalTimeRegisters += register.TotalTime
		if register.MaxQueueTime > maxQueueTimeRegisters {
			maxQueueTimeRegisters = register.MaxQueueTime
		}
	}
	avgQueueTimeRegisters := totalTimeRegisters / time.Duration(totalCarsRegisters)

	fmt.Println("Registers:")
	fmt.Printf("	total_cars: %d\n", totalCarsRegisters)
	fmt.Printf("	total_time: %s\n", totalTimeRegisters)
	fmt.Printf("	avg_queue_time: %s\n", avgQueueTimeRegisters)
	fmt.Printf("	max_queue_time: %s\n", maxQueueTimeRegisters)

}

func main() {
	configFile := "config.yaml"

	simulation, err := initiateSimulation(configFile)

	if err != nil {
        panic(err)
    }

	simulation.runSimulation()
	simulation.printResults()

}