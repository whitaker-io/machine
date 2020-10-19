package machine

import (
	"context"
	"fmt"
	"reflect"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	t.Run("Test_Machine_Run_route_error", func(t *testing.T) {
		count := 100000
		out := make(chan []map[string]interface{})
		m := New("machine_id", "machine", true, func(c context.Context) chan []map[string]interface{} {
			channel := make(chan []map[string]interface{})

			go func() {
				for i := 0; i < count; i++ {
					channel <- testList1
				}
			}()

			return channel
		})

		node := m.Then("node_id", "node", true, func(m map[string]interface{}) error {
			if _, ok := m["name"]; !ok {
				t.Errorf("machine.begin() nissing name %v", m)
				return fmt.Errorf("node.cascade() incorrect data have %v want %v", m, "name field")
			}
			return nil
		})

		router := node.Route("route_id", "route", true, RouterError{}.Handler)

		router.TerminateLeft("terminus_id", "terminus", true, func(list []map[string]interface{}) error {
			for i, packet := range list {
				if !reflect.DeepEqual(packet, testList1[i]) {
					t.Errorf("machine.begin() incorrect data have %v want %v", packet, testList1[i])
				}
			}
			out <- list
			return fmt.Errorf("error everything")
		})

		router.TerminateRight("terminus_id", "terminus", true, func(list []map[string]interface{}) error {
			t.Errorf(" no errors expected")
			return nil
		})

		machine := m.Build(func(id, name string, payload []*Packet) {})

		ctx, cancel := context.WithCancel(context.Background())

		if err := machine.Run(ctx); err != nil {
			t.Error(err)
		}

		x := map[string][]*Packet{
			"node_id": testPayload,
		}

		go func() {
			for {
				machine.Inject(x)
			}
		}()

		<-time.After(time.Second / 3)

		cancel()

		<-time.After(1 * time.Second)
	})
}

func TestNew2(t *testing.T) {
	t.Run("Test_Machine_Run_route_error", func(t *testing.T) {
		count := 100000
		out := make(chan []map[string]interface{})
		m := New("machine_id", "machine", false, func(c context.Context) chan []map[string]interface{} {
			channel := make(chan []map[string]interface{})

			go func() {
				for i := 0; i < count; i++ {
					channel <- testList1
				}
			}()

			return channel
		})

		router := m.Route("route_id", "route", false, RouterError{}.Handler)

		left := router.ThenLeft("node_id", "node", false, func(m map[string]interface{}) error {
			if _, ok := m["name"]; !ok {
				t.Errorf("machine.begin() nissing name %v", m)
				return fmt.Errorf("node.cascade() incorrect data have %v want %v", m, "name field")
			}
			return nil
		})

		right := router.ThenRight("node_id", "node", false, func(m map[string]interface{}) error {
			if _, ok := m["name"]; !ok {
				t.Errorf("machine.begin() nissing name %v", m)
				return fmt.Errorf("node.cascade() incorrect data have %v want %v", m, "name field")
			}
			return nil
		})

		left.Terminate("terminus_id", "terminus", false, func(list []map[string]interface{}) error {
			for i, packet := range list {
				if !reflect.DeepEqual(packet, testList1[i]) {
					t.Errorf("machine.begin() incorrect data have %v want %v", packet, testList1[i])
				}
			}
			out <- list
			return fmt.Errorf("error everything")
		})

		right.Terminate("terminus_id", "terminus", false, func(list []map[string]interface{}) error {
			t.Errorf(" no errors expected")
			return nil
		})

		machine := m.Build(func(id, name string, payload []*Packet) {})

		ctx, cancel := context.WithCancel(context.Background())

		if err := machine.Run(ctx); err != nil {
			t.Error(err)
		}

		x := map[string][]*Packet{
			"node_id": testPayload,
		}

		go func() {
			for {
				machine.Inject(x)
			}
		}()

		<-time.After(time.Second / 3)

		cancel()

		<-time.After(1 * time.Second)
	})
}

func TestNew3(t *testing.T) {
	t.Run("Test_Machine_Run_route_error", func(t *testing.T) {
		count := 100000
		out := make(chan []map[string]interface{})
		m := New("machine_id", "machine", false, func(c context.Context) chan []map[string]interface{} {
			channel := make(chan []map[string]interface{})

			go func() {
				for i := 0; i < count; i++ {
					channel <- testList1
				}
			}()

			return channel
		})

		router := m.Route("route_id", "route", false, RouterError{}.Handler)

		router.ThenLeft("node_id", "node", false, func(m map[string]interface{}) error {
			if _, ok := m["name"]; !ok {
				t.Errorf("machine.begin() nissing name %v", m)
				return fmt.Errorf("node.cascade() incorrect data have %v want %v", m, "name field")
			}
			return nil
		}).Then("node_id", "node", false, func(m map[string]interface{}) error {
			if _, ok := m["name"]; !ok {
				t.Errorf("machine.begin() nissing name %v", m)
				return fmt.Errorf("node.cascade() incorrect data have %v want %v", m, "name field")
			}
			return nil
		}).Terminate("terminus_id", "terminus", false, func(list []map[string]interface{}) error {
			for i, packet := range list {
				if !reflect.DeepEqual(packet, testList1[i]) {
					t.Errorf("machine.begin() incorrect data have %v want %v", packet, testList1[i])
				}
			}
			out <- list
			return fmt.Errorf("error everything")
		})

		router2 := router.RouteRight("route_id", "route", false, RouterError{}.Handler)

		router2.TerminateLeft("terminus_id", "terminus", false, func(list []map[string]interface{}) error {
			t.Errorf(" no errors expected")
			return nil
		})

		router2.TerminateRight("terminus_id", "terminus", false, func(list []map[string]interface{}) error {
			t.Errorf(" no errors expected")
			return nil
		})

		machine := m.Build(func(id, name string, payload []*Packet) {})

		ctx, cancel := context.WithCancel(context.Background())

		if err := machine.Run(ctx); err != nil {
			t.Error(err)
		}

		x := map[string][]*Packet{
			"node_id": testPayload,
		}

		go func() {
			for {
				machine.Inject(x)
			}
		}()

		<-time.After(time.Second / 3)

		cancel()

		<-time.After(1 * time.Second)
	})
}

func TestNew4(t *testing.T) {
	t.Run("Test_Machine_Run_route_error", func(t *testing.T) {
		count := 100000
		out := make(chan []map[string]interface{})

		m1 := New("machine_id", "machine", false, func(c context.Context) chan []map[string]interface{} {
			channel := make(chan []map[string]interface{})

			go func() {
				for i := 0; i < count; i++ {
					channel <- testList1
				}
			}()

			return channel
		}).Terminate("terminus_id", "terminus", false, func(list []map[string]interface{}) error {
			for i, packet := range list {
				if !reflect.DeepEqual(packet, testList1[i]) {
					t.Errorf("machine.begin() incorrect data have %v want %v", packet, testList1[i])
				}
			}
			out <- list
			return fmt.Errorf("error everything")
		})

		m := New("machine_id", "machine", false, func(c context.Context) chan []map[string]interface{} {
			channel := make(chan []map[string]interface{})

			go func() {
				for i := 0; i < count; i++ {
					channel <- testList1
				}
			}()

			return channel
		})

		router := m.Route("route_id", "route", false, RouterError{}.Handler)

		router.ThenRight("node_id", "node", false, func(m map[string]interface{}) error {
			if _, ok := m["name"]; !ok {
				t.Errorf("machine.begin() nissing name %v", m)
				return fmt.Errorf("node.cascade() incorrect data have %v want %v", m, "name field")
			}
			return nil
		}).Then("node_id", "node", false, func(m map[string]interface{}) error {
			if _, ok := m["name"]; !ok {
				t.Errorf("machine.begin() nissing name %v", m)
				return fmt.Errorf("node.cascade() incorrect data have %v want %v", m, "name field")
			}
			return nil
		}).Terminate("terminus_id", "terminus", false, func(list []map[string]interface{}) error {
			for i, packet := range list {
				if !reflect.DeepEqual(packet, testList1[i]) {
					t.Errorf("machine.begin() incorrect data have %v want %v", packet, testList1[i])
				}
			}
			out <- list
			return fmt.Errorf("error everything")
		})

		router2 := router.RouteLeft("route_id", "route", false, RouterError{}.Handler)

		router2.TerminateLeft("terminus_id", "terminus", false, func(list []map[string]interface{}) error {
			t.Errorf(" no errors expected")
			return nil
		})

		router2.TerminateRight("terminus_id", "terminus", false, func(list []map[string]interface{}) error {
			t.Errorf(" no errors expected")
			return nil
		})

		machine := m.Build(func(id, name string, payload []*Packet) {})
		machine1 := m1.Build(func(id, name string, payload []*Packet) {})

		ctx, cancel := context.WithCancel(context.Background())

		if err := machine.Run(ctx); err != nil {
			t.Error(err)
		} else if err := machine1.Run(ctx); err != nil {
			t.Error(err)
		}

		x := map[string][]*Packet{
			"node_id": testPayload,
		}

		go func() {
			for {
				machine.Inject(x)
			}
		}()

		<-time.After(time.Second / 3)

		cancel()

		<-time.After(1 * time.Second)
	})
}