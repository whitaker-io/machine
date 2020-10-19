package machine

import (
	"context"
	"fmt"
	"reflect"
	"testing"
	"time"
)

var testList1 = []map[string]interface{}{
	{
		"name":  "data0",
		"value": 0,
	},
	{
		"name":  "data1",
		"value": 1,
	},
	{
		"name":  "data2",
		"value": 2,
	},
	{
		"name":  "data3",
		"value": 3,
	},
}

var testList2 = []map[string]interface{}{
	{
		"name":  "data4",
		"value": 4,
	},
	{
		"name":  "data5",
		"value": 5,
	},
	{
		"name":  "data6",
		"value": 6,
	},
	{
		"name":  "data7",
		"value": 7,
	},
}

var testPayload = []*Packet{
	{
		ID: "ID_0",
		Data: map[string]interface{}{
			"name":  "data0",
			"value": 0,
		},
	},
	{
		ID: "ID_1",
		Data: map[string]interface{}{
			"name":  "data1",
			"value": 1,
		},
	},
	{
		ID: "ID_2",
		Data: map[string]interface{}{
			"name":  "data2",
			"value": 2,
		},
	},
	{
		ID: "ID_3",
		Data: map[string]interface{}{
			"name":  "data3",
			"value": 3,
		},
	},
}

func TestMachine_begin(t *testing.T) {
	t.Run("machine_begin", func(t *testing.T) {
		m := &Machine{
			info: info{
				id:   "machine_id",
				name: "machine_name",
				recorder: func(id, name string, payload []*Packet) {

				},
				fifo: false,
			},
			initium: func(ctx context.Context) chan []map[string]interface{} {
				channel := make(chan []map[string]interface{})

				go func() {
					channel <- testList1
					channel <- testList2
				}()

				return channel
			},
		}

		out := m.begin(context.Background())

		list1 := <-out.channel
		list2 := <-out.channel

		for i, packet := range list1 {
			if !reflect.DeepEqual(packet.Data, testList1[i]) {
				t.Errorf("machine.begin() incorrect data have %v want %v", packet, testList1[i])
			}
		}

		for i, packet := range list2 {
			if !reflect.DeepEqual(packet.Data, testList2[i]) {
				t.Errorf("machine.begin() incorrect data have %v want %v", packet, testList1[i])
			}
		}
	})
}

func TestMachine_begin_continuous_in_order(t *testing.T) {
	count := 100000
	t.Run("TestMachine_begin_continuous_in_order", func(t *testing.T) {
		m := &Machine{
			info: info{
				id:   "machine_id",
				name: "machine_name",
				recorder: func(id, name string, payload []*Packet) {

				},
				fifo: false,
			},
			initium: func(ctx context.Context) chan []map[string]interface{} {
				channel := make(chan []map[string]interface{})

				go func() {
					for i := 0; i < count; i++ {
						channel <- testList1
					}
					channel <- testList2
				}()

				return channel
			},
		}

		out := m.begin(context.Background())

		for i := 0; i < count; i++ {
			list1 := <-out.channel
			for i, packet := range list1 {
				if !reflect.DeepEqual(packet.Data, testList1[i]) {
					t.Errorf("machine.begin() incorrect data have %v want %v", packet, testList1[i])
				}
			}
		}

		list2 := <-out.channel

		for i, packet := range list2 {
			if !reflect.DeepEqual(packet.Data, testList2[i]) {
				t.Errorf("machine.begin() incorrect data have %v want %v", packet, testList1[i])
			}
		}
	})
}

func Test_Machine_Run(t *testing.T) {
	t.Run("Test_Machine_Run", func(t *testing.T) {
		count := 100000
		out := make(chan []map[string]interface{})
		term := &termination{
			info: info{
				id:   "node_id",
				name: "node_name",
				recorder: func(id, name string, payload []*Packet) {

				},
				fifo: false,
			},
			terminus: func(list []map[string]interface{}) error {
				for i, packet := range list {
					if !reflect.DeepEqual(packet, testList1[i]) {
						t.Errorf("machine.begin() incorrect data have %v want %v", packet, testList1[i])
					}
				}
				out <- list
				return nil
			},
		}

		n := &node{
			info: info{
				id:   "node_id",
				name: "node_name",
				recorder: func(id, name string, payload []*Packet) {

				},
				fifo: false,
			},
			processus: func(m map[string]interface{}) error {
				if _, ok := m["name"]; !ok {
					return fmt.Errorf("node.cascade() incorrect data have %v want %v", m, "name field")
				}
				return nil
			},
			child: term,
		}

		m := &Machine{
			info: info{
				id:   "machine_id",
				name: "machine_name",
				recorder: func(id, name string, payload []*Packet) {

				},
				fifo: false,
			},
			initium: func(ctx context.Context) chan []map[string]interface{} {
				channel := make(chan []map[string]interface{})

				go func() {
					for i := 0; i < count; i++ {
						channel <- testList1
					}
				}()

				return channel
			},
			child: n,
			nodes: map[string]*node{},
		}

		if err := m.Run(context.Background()); err != nil {
			t.Error(err)
		}

		for i := 0; i < count; i++ {
			list1 := <-out
			for i, packet := range list1 {
				if !reflect.DeepEqual(packet, testList1[i]) {
					t.Errorf("machine.begin() incorrect data have %v want %v", packet, testList1[i])
				}
			}
		}
	})
}

func Test_Machine_Run_error(t *testing.T) {
	t.Run("Test_Machine_Run_error", func(t *testing.T) {
		m := &Machine{
			info: info{
				id:   "machine_id",
				name: "machine_name",
				recorder: func(id, name string, payload []*Packet) {

				},
				fifo: false,
			},
			initium: func(ctx context.Context) chan []map[string]interface{} {
				channel := make(chan []map[string]interface{})

				return channel
			},
			nodes: map[string]*node{},
		}

		if err := m.Run(context.Background()); err == nil {
			t.Errorf("machine.Run() expected error for missing child")
		}
	})
}

func Test_Machine_Run_node_error(t *testing.T) {
	t.Run("Test_Machine_Run_node_error", func(t *testing.T) {
		n := &node{
			info: info{
				id:   "node_id",
				name: "node_name",
				recorder: func(id, name string, payload []*Packet) {

				},
				fifo: false,
			},
			processus: func(m map[string]interface{}) error {
				if _, ok := m["name"]; !ok {
					return fmt.Errorf("node.cascade() incorrect data have %v want %v", m, "name field")
				}
				return nil
			},
		}

		m := &Machine{
			info: info{
				id:   "machine_id",
				name: "machine_name",
				recorder: func(id, name string, payload []*Packet) {

				},
				fifo: false,
			},
			initium: func(ctx context.Context) chan []map[string]interface{} {
				channel := make(chan []map[string]interface{})
				return channel
			},
			child: n,
			nodes: map[string]*node{},
		}

		if err := m.Run(context.Background()); err == nil {
			t.Errorf("machine.Run() expected error for missing child")
		}
	})
}

func Test_Machine_ID(t *testing.T) {
	t.Run("Test_Machine_ID", func(t *testing.T) {
		m := &Machine{
			info: info{
				id:   "machine_id",
				name: "machine_name",
				recorder: func(id, name string, payload []*Packet) {

				},
				fifo: false,
			},
			initium: func(ctx context.Context) chan []map[string]interface{} {
				channel := make(chan []map[string]interface{})

				return channel
			},
			nodes: map[string]*node{},
		}

		if m.ID() != "machine_id" {
			t.Errorf("machine.Run() error incorrect ID have %s want %s", m.ID(), "machine_id")
		}
	})
}

func Test_Machine_Inject(t *testing.T) {
	t.Run("Test_Machine_Inject", func(t *testing.T) {
		count := 100000
		out := make(chan []map[string]interface{})
		term := &termination{
			info: info{
				id:   "node_id",
				name: "node_name",
				recorder: func(id, name string, payload []*Packet) {

				},
				fifo: false,
			},
			terminus: func(list []map[string]interface{}) error {
				for i, packet := range list {
					if !reflect.DeepEqual(packet, testList1[i]) {
						t.Errorf("machine.begin() incorrect data have %v want %v", packet, testList1[i])
					}
				}
				out <- list
				return fmt.Errorf("test error")
			},
		}

		n := &node{
			info: info{
				id:   "node_id",
				name: "node_name",
				recorder: func(id, name string, payload []*Packet) {

				},
				fifo: false,
			},
			processus: func(m map[string]interface{}) error {
				if _, ok := m["name"]; !ok {
					return fmt.Errorf("node.cascade() incorrect data have %v want %v", m, "name field")
				}
				return nil
			},
			child: term,
		}

		m := &Machine{
			info: info{
				id:   "machine_id",
				name: "machine_name",
				recorder: func(id, name string, payload []*Packet) {

				},
				fifo: false,
			},
			initium: func(ctx context.Context) chan []map[string]interface{} {
				channel := make(chan []map[string]interface{})

				go func() {
					for i := 0; i < count; i++ {
						channel <- testList1
					}
				}()

				return channel
			},
			child: n,
			nodes: map[string]*node{},
		}

		if err := m.Run(context.Background()); err != nil {
			t.Error(err)
		}

		x := map[string][]*Packet{
			"node_id": testPayload,
		}

		m.Inject(x)

		list1 := <-out
		for i, packet := range list1 {
			if !reflect.DeepEqual(packet, testList1[i]) {
				t.Errorf("machine.begin() incorrect data have %v want %v", packet, testList1[i])
			}
		}
	})
}

func Test_Machine_Run_cancel(t *testing.T) {
	t.Run("Test_Machine_Run_cancel", func(t *testing.T) {
		count := 100000
		out := make(chan []map[string]interface{})
		term := &termination{
			info: info{
				id:   "node_id",
				name: "node_name",
				recorder: func(id, name string, payload []*Packet) {

				},
				fifo: false,
			},
			terminus: func(list []map[string]interface{}) error {
				for i, packet := range list {
					if !reflect.DeepEqual(packet, testList1[i]) {
						t.Errorf("machine.begin() incorrect data have %v want %v", packet, testList1[i])
					}
				}
				out <- list
				return nil
			},
		}

		n := &node{
			info: info{
				id:   "node_id",
				name: "node_name",
				recorder: func(id, name string, payload []*Packet) {

				},
				fifo: false,
			},
			processus: func(m map[string]interface{}) error {
				if _, ok := m["name"]; !ok {
					t.Errorf("machine.begin() nissing name %v", m)
					return fmt.Errorf("node.cascade() incorrect data have %v want %v", m, "name field")
				}
				return nil
			},
			child: term,
		}

		m := &Machine{
			info: info{
				id:   "machine_id",
				name: "machine_name",
				recorder: func(id, name string, payload []*Packet) {

				},
				fifo: false,
			},
			initium: func(ctx context.Context) chan []map[string]interface{} {
				channel := make(chan []map[string]interface{})

				go func() {
					for i := 0; i < count; i++ {
						channel <- testList1
					}
				}()

				return channel
			},
			child: n,
			nodes: map[string]*node{},
		}

		ctx, cancel := context.WithCancel(context.Background())

		if err := m.Run(ctx); err != nil {
			t.Error(err)
		}

		x := map[string][]*Packet{
			"node_id": testPayload,
		}

		go func() {
			for {
				m.Inject(x)
			}
		}()

		<-time.After(time.Second / 3)

		cancel()

		<-time.After(1 * time.Second)
	})
}

func Test_Machine_Run_cancel_fifo(t *testing.T) {
	t.Run("Test_Machine_Run_cancel_fifo", func(t *testing.T) {
		count := 100000
		out := make(chan []map[string]interface{})
		term := &termination{
			info: info{
				id:   "node_id",
				name: "node_name",
				recorder: func(id, name string, payload []*Packet) {

				},
				fifo: true,
			},
			terminus: func(list []map[string]interface{}) error {
				for i, packet := range list {
					if !reflect.DeepEqual(packet, testList1[i]) {
						t.Errorf("machine.begin() incorrect data have %v want %v", packet, testList1[i])
					}
				}
				out <- list
				return nil
			},
		}

		n := &node{
			info: info{
				id:   "node_id",
				name: "node_name",
				recorder: func(id, name string, payload []*Packet) {

				},
				fifo: true,
			},
			processus: func(m map[string]interface{}) error {
				if _, ok := m["name"]; !ok {
					t.Errorf("machine.begin() nissing name %v", m)
					return fmt.Errorf("node.cascade() incorrect data have %v want %v", m, "name field")
				}
				return nil
			},
			child: term,
		}

		m := &Machine{
			info: info{
				id:   "machine_id",
				name: "machine_name",
				recorder: func(id, name string, payload []*Packet) {

				},
				fifo: true,
			},
			initium: func(ctx context.Context) chan []map[string]interface{} {
				channel := make(chan []map[string]interface{})

				go func() {
					for i := 0; i < count; i++ {
						channel <- testList1
					}
				}()

				return channel
			},
			child: n,
			nodes: map[string]*node{},
		}

		ctx, cancel := context.WithCancel(context.Background())

		if err := m.Run(ctx); err != nil {
			t.Error(err)
		}

		x := map[string][]*Packet{
			"node_id": testPayload,
		}

		go func() {
			for {
				m.Inject(x)
			}
		}()

		<-time.After(time.Second / 3)

		cancel()

		<-time.After(1 * time.Second)
	})
}

func Test_Machine_Run_route(t *testing.T) {
	t.Run("Test_Machine_Run_route", func(t *testing.T) {
		count := 100000
		out := make(chan []map[string]interface{})
		term := &termination{
			info: info{
				id:   "term_id",
				name: "term_name",
				recorder: func(id, name string, payload []*Packet) {

				},
				fifo: false,
			},
			terminus: func(list []map[string]interface{}) error {
				for i, packet := range list {
					if !reflect.DeepEqual(packet, testList1[i]) {
						t.Errorf("machine.begin() incorrect data have %v want %v", packet, testList1[i])
					}
				}
				out <- list
				return nil
			},
		}

		term2 := &termination{
			info: info{
				id:   "term_id",
				name: "term_name",
				recorder: func(id, name string, payload []*Packet) {

				},
				fifo: false,
			},
			terminus: func(list []map[string]interface{}) error {
				t.Errorf("machine.begin() route shouldnt terminate here")
				return nil
			},
		}

		var r RouterRule = func(map[string]interface{}) bool {
			return true
		}

		route := (RouteHandler(r.Handler)).convert("rule_id", "rule_name", false)

		route.left = term
		route.right = term2

		m := &Machine{
			info: info{
				id:   "machine_id",
				name: "machine_name",
				recorder: func(id, name string, payload []*Packet) {

				},
				fifo: false,
			},
			initium: func(ctx context.Context) chan []map[string]interface{} {
				channel := make(chan []map[string]interface{})

				go func() {
					for i := 0; i < count; i++ {
						channel <- testList1
					}
				}()

				return channel
			},
			child: route,
			nodes: map[string]*node{},
		}

		if err := m.Run(context.Background()); err != nil {
			t.Error(err)
		}

		for i := 0; i < count; i++ {
			list1 := <-out
			for i, packet := range list1 {
				if !reflect.DeepEqual(packet, testList1[i]) {
					t.Errorf("machine.begin() incorrect data have %v want %v", packet, testList1[i])
				}
			}
		}
	})
}

func Test_Machine_Run_route_right(t *testing.T) {
	t.Run("Test_Machine_Run_route_right", func(t *testing.T) {
		count := 100000
		out := make(chan []map[string]interface{})
		term := &termination{
			info: info{
				id:   "term_id",
				name: "term_name",
				recorder: func(id, name string, payload []*Packet) {

				},
				fifo: false,
			},
			terminus: func(list []map[string]interface{}) error {
				for i, packet := range list {
					if !reflect.DeepEqual(packet, testList1[i]) {
						t.Errorf("machine.begin() incorrect data have %v want %v", packet, testList1[i])
					}
				}
				out <- list
				return nil
			},
		}

		term2 := &termination{
			info: info{
				id:   "term_id",
				name: "term_name",
				recorder: func(id, name string, payload []*Packet) {

				},
				fifo: false,
			},
			terminus: func(list []map[string]interface{}) error {
				t.Errorf("machine.begin() route shouldnt terminate here")
				return nil
			},
		}

		var r RouterRule = func(map[string]interface{}) bool {
			return false
		}

		route := (RouteHandler(r.Handler)).convert("rule_id", "rule_name", false)

		route.left = term2
		route.right = term

		m := &Machine{
			info: info{
				id:   "machine_id",
				name: "machine_name",
				recorder: func(id, name string, payload []*Packet) {

				},
				fifo: false,
			},
			initium: func(ctx context.Context) chan []map[string]interface{} {
				channel := make(chan []map[string]interface{})

				go func() {
					for i := 0; i < count; i++ {
						channel <- testList1
					}
				}()

				return channel
			},
			child: route,
			nodes: map[string]*node{},
		}

		if err := m.Run(context.Background()); err != nil {
			t.Error(err)
		}

		for i := 0; i < count; i++ {
			list1 := <-out
			for i, packet := range list1 {
				if !reflect.DeepEqual(packet, testList1[i]) {
					t.Errorf("machine.begin() incorrect data have %v want %v", packet, testList1[i])
				}
			}
		}
	})
}

func Test_Machine_Run_route_error(t *testing.T) {
	t.Run("Test_Machine_Run_route_error", func(t *testing.T) {
		count := 100000
		out := make(chan []map[string]interface{})
		term := &termination{
			info: info{
				id:   "term_id",
				name: "term_name",
				recorder: func(id, name string, payload []*Packet) {

				},
				fifo: false,
			},
			terminus: func(list []map[string]interface{}) error {
				for i, packet := range list {
					if !reflect.DeepEqual(packet, testList1[i]) {
						t.Errorf("machine.begin() incorrect data have %v want %v", packet, testList1[i])
					}
				}
				out <- list
				return nil
			},
		}

		term2 := &termination{
			info: info{
				id:   "term_id",
				name: "term_name",
				recorder: func(id, name string, payload []*Packet) {

				},
				fifo: false,
			},
			terminus: func(list []map[string]interface{}) error {
				t.Errorf("machine.begin() route shouldnt terminate here")
				return nil
			},
		}

		r := RouterError{}

		route := (RouteHandler(r.Handler)).convert("rule_id", "rule_name", false)

		route.left = term
		route.right = term2

		m := &Machine{
			info: info{
				id:   "machine_id",
				name: "machine_name",
				recorder: func(id, name string, payload []*Packet) {

				},
				fifo: false,
			},
			initium: func(ctx context.Context) chan []map[string]interface{} {
				channel := make(chan []map[string]interface{})

				go func() {
					for i := 0; i < count; i++ {
						channel <- testList1
					}
				}()

				return channel
			},
			child: route,
			nodes: map[string]*node{},
		}

		if err := m.Run(context.Background()); err != nil {
			t.Error(err)
		}

		for i := 0; i < count; i++ {
			list1 := <-out
			for i, packet := range list1 {
				if !reflect.DeepEqual(packet, testList1[i]) {
					t.Errorf("machine.begin() incorrect data have %v want %v", packet, testList1[i])
				}
			}
		}
	})
}

func Test_Machine_Run_route_error_right(t *testing.T) {
	t.Run("Test_Machine_Run_route_error_right", func(t *testing.T) {
		count := 100000
		out := make(chan []map[string]interface{})
		term := &termination{
			info: info{
				id:   "term_id",
				name: "term_name",
				recorder: func(id, name string, payload []*Packet) {

				},
				fifo: false,
			},
			terminus: func(list []map[string]interface{}) error {
				for i, packet := range list {
					if !reflect.DeepEqual(packet, testList1[i]) {
						t.Errorf("machine.begin() incorrect data have %v want %v", packet, testList1[i])
					}
				}
				out <- list
				return nil
			},
		}

		term2 := &termination{
			info: info{
				id:   "term_id",
				name: "term_name",
				recorder: func(id, name string, payload []*Packet) {

				},
				fifo: false,
			},
			terminus: func(list []map[string]interface{}) error {
				t.Errorf("machine.begin() route shouldnt terminate here")
				return nil
			},
		}

		r := RouterError{}

		route := (RouteHandler(r.Handler)).convert("rule_id", "rule_name", false)

		route.left = term2
		route.right = term

		n := &node{
			info: info{
				id:   "node_id",
				name: "node_name",
				recorder: func(id, name string, payload []*Packet) {

				},
				fifo: true,
			},
			processus: func(m map[string]interface{}) error {
				return fmt.Errorf("test error")
			},
			child: route,
		}

		m := &Machine{
			info: info{
				id:   "machine_id",
				name: "machine_name",
				recorder: func(id, name string, payload []*Packet) {

				},
				fifo: false,
			},
			initium: func(ctx context.Context) chan []map[string]interface{} {
				channel := make(chan []map[string]interface{})

				go func() {
					for i := 0; i < count; i++ {
						channel <- testList1
					}
				}()

				return channel
			},
			child: n,
			nodes: map[string]*node{},
		}

		if err := m.Run(context.Background()); err != nil {
			t.Error(err)
		}

		for i := 0; i < count; i++ {
			list1 := <-out
			for i, packet := range list1 {
				if !reflect.DeepEqual(packet, testList1[i]) {
					t.Errorf("machine.begin() incorrect data have %v want %v", packet, testList1[i])
				}
			}
		}
	})
}

func Test_Machine_Run_route_duplicate(t *testing.T) {
	t.Run("Test_Machine_Run_route_duplicate", func(t *testing.T) {
		count := 100000
		out := make(chan []map[string]interface{})
		term := &termination{
			info: info{
				id:   "term_id",
				name: "term_name",
				recorder: func(id, name string, payload []*Packet) {

				},
				fifo: false,
			},
			terminus: func(list []map[string]interface{}) error {
				for i, packet := range list {
					if !reflect.DeepEqual(packet, testList1[i]) {
						t.Errorf("machine.begin() incorrect data have %v want %v", packet, testList1[i])
					}
				}
				out <- list
				return nil
			},
		}

		r := RouterDuplicate{}

		route := (RouteHandler(r.Handler)).convert("rule_id", "rule_name", false)

		route.left = term
		route.right = term

		m := &Machine{
			info: info{
				id:   "machine_id",
				name: "machine_name",
				recorder: func(id, name string, payload []*Packet) {

				},
				fifo: false,
			},
			initium: func(ctx context.Context) chan []map[string]interface{} {
				channel := make(chan []map[string]interface{})

				go func() {
					for i := 0; i < count; i++ {
						channel <- testList1
					}
				}()

				return channel
			},
			child: route,
			nodes: map[string]*node{},
		}

		if err := m.Run(context.Background()); err != nil {
			t.Error(err)
		}

		for i := 0; i < count*2; i++ {
			list1 := <-out
			for i, packet := range list1 {
				if !reflect.DeepEqual(packet, testList1[i]) {
					t.Errorf("machine.begin() incorrect data have %v want %v", packet, testList1[i])
				}
			}
		}
	})
}

func Test_Machine_Run_route_duplicate_no_child(t *testing.T) {
	t.Run("Test_Machine_Run_route_duplicate_no_child", func(t *testing.T) {
		r := RouterDuplicate{}

		route := (RouteHandler(r.Handler)).convert("rule_id", "rule_name", false)

		m := &Machine{
			info: info{
				id:   "machine_id",
				name: "machine_name",
				recorder: func(id, name string, payload []*Packet) {

				},
				fifo: false,
			},
			initium: func(ctx context.Context) chan []map[string]interface{} {
				channel := make(chan []map[string]interface{})

				return channel
			},
			child: route,
			nodes: map[string]*node{},
		}

		if err := m.Run(context.Background()); err == nil {
			t.Errorf("Test_Machine_Run_route_duplicate_no_child child found")
		}
	})
}

func Test_Machine_Run_route_duplicate_left_no_child(t *testing.T) {
	t.Run("Test_Machine_Run_route_duplicate_left_no_child", func(t *testing.T) {
		out := make(chan []map[string]interface{})
		term := &termination{
			info: info{
				id:   "term_id",
				name: "term_name",
				recorder: func(id, name string, payload []*Packet) {

				},
				fifo: false,
			},
			terminus: func(list []map[string]interface{}) error {
				for i, packet := range list {
					if !reflect.DeepEqual(packet, testList1[i]) {
						t.Errorf("machine.begin() incorrect data have %v want %v", packet, testList1[i])
					}
				}
				out <- list
				return nil
			},
		}
		n := &node{
			info: info{
				id:   "node_id",
				name: "node_name",
				recorder: func(id, name string, payload []*Packet) {

				},
				fifo: true,
			},
			processus: func(m map[string]interface{}) error {
				return fmt.Errorf("test error")
			},
			child: term,
		}

		n2 := &node{
			info: info{
				id:   "node_id",
				name: "node_name",
				recorder: func(id, name string, payload []*Packet) {

				},
				fifo: true,
			},
			processus: func(m map[string]interface{}) error {
				return fmt.Errorf("test error")
			},
		}

		r := RouterDuplicate{}

		route := (RouteHandler(r.Handler)).convert("rule_id", "rule_name", false)

		route.left = n2
		route.right = n

		m := &Machine{
			info: info{
				id:   "machine_id",
				name: "machine_name",
				recorder: func(id, name string, payload []*Packet) {

				},
				fifo: false,
			},
			initium: func(ctx context.Context) chan []map[string]interface{} {
				channel := make(chan []map[string]interface{})

				return channel
			},
			child: route,
			nodes: map[string]*node{},
		}

		if err := m.Run(context.Background()); err == nil {
			t.Errorf("Test_Machine_Run_route_duplicate_left_no_child child found")
		}
	})
}

func Test_Machine_Run_route_duplicate_right_no_child(t *testing.T) {
	t.Run("Test_Machine_Run_route_duplicate_right_no_child", func(t *testing.T) {
		out := make(chan []map[string]interface{})
		term := &termination{
			info: info{
				id:   "term_id",
				name: "term_name",
				recorder: func(id, name string, payload []*Packet) {

				},
				fifo: false,
			},
			terminus: func(list []map[string]interface{}) error {
				for i, packet := range list {
					if !reflect.DeepEqual(packet, testList1[i]) {
						t.Errorf("machine.begin() incorrect data have %v want %v", packet, testList1[i])
					}
				}
				out <- list
				return nil
			},
		}
		n := &node{
			info: info{
				id:   "node_id",
				name: "node_name",
				recorder: func(id, name string, payload []*Packet) {

				},
				fifo: true,
			},
			processus: func(m map[string]interface{}) error {
				return fmt.Errorf("test error")
			},
			child: term,
		}

		n2 := &node{
			info: info{
				id:   "node_id",
				name: "node_name",
				recorder: func(id, name string, payload []*Packet) {

				},
				fifo: true,
			},
			processus: func(m map[string]interface{}) error {
				return fmt.Errorf("test error")
			},
		}

		r := RouterDuplicate{}

		route := (RouteHandler(r.Handler)).convert("rule_id", "rule_name", false)

		route.left = n
		route.right = n2

		m := &Machine{
			info: info{
				id:   "machine_id",
				name: "machine_name",
				recorder: func(id, name string, payload []*Packet) {

				},
				fifo: false,
			},
			initium: func(ctx context.Context) chan []map[string]interface{} {
				channel := make(chan []map[string]interface{})

				return channel
			},
			child: route,
			nodes: map[string]*node{},
		}

		if err := m.Run(context.Background()); err == nil {
			t.Errorf("Test_Machine_Run_route_duplicate_right_no_child child found")
		}
	})
}

func Test_Machine_Run_route_duplicate_missing_machines(t *testing.T) {
	t.Run("Test_Machine_Run_route_duplicate_missing_machines", func(t *testing.T) {
		out := make(chan []map[string]interface{})
		term := &termination{
			info: info{
				id:   "term_id",
				name: "term_name",
				recorder: func(id, name string, payload []*Packet) {

				},
				fifo: false,
			},
			terminus: func(list []map[string]interface{}) error {
				for i, packet := range list {
					if !reflect.DeepEqual(packet, testList1[i]) {
						t.Errorf("machine.begin() incorrect data have %v want %v", packet, testList1[i])
					}
				}
				out <- list
				return nil
			},
		}
		n := &node{
			info: info{
				id:   "node_id",
				name: "node_name",
				recorder: func(id, name string, payload []*Packet) {

				},
				fifo: true,
			},
			processus: func(m map[string]interface{}) error {
				return fmt.Errorf("test error")
			},
			child: term,
		}

		r := RouterDuplicate{}

		route := (RouteHandler(r.Handler)).convert("rule_id", "rule_name", false)

		if err := route.cascade(context.Background(), nil, nil); err == nil {
			t.Errorf("Test_Machine_Run_route_duplicate_missing_machines route machine found")
		}

		if err := n.cascade(context.Background(), nil, nil); err == nil {
			t.Errorf("Test_Machine_Run_route_duplicate_missing_machines node machine found")
		}

		if err := term.cascade(context.Background(), nil, nil); err == nil {
			t.Errorf("Test_Machine_Run_route_duplicate_missing_machines terminus machine found")
		}
	})
}
