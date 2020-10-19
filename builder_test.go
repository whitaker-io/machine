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

func Test_New(t *testing.T) {
	t.Run("Test_New", func(t *testing.T) {
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
		}).Then(
			NewVertex("node_id1", "node1", false, func(m map[string]interface{}) error {
				if _, ok := m["name"]; !ok {
					t.Errorf("machine.begin() nissing name %v", m)
					return fmt.Errorf("node.cascade() incorrect data have %v want %v", m, "name field")
				}
				return nil
			}).Then(
				NewVertex("node_id2", "node2", false, func(m map[string]interface{}) error {
					if _, ok := m["name"]; !ok {
						t.Errorf("machine.begin() nissing name %v", m)
						return fmt.Errorf("node.cascade() incorrect data have %v want %v", m, "name field")
					}
					return nil
				}).Route(
					NewRouter("route_id", "route", false, RouterError{}.Handler).
						RouteLeft(
							NewRouter("route_id", "route", false, RouterError{}.Handler).
								ThenLeft(
									NewVertex("node_id3", "node3", false, func(m map[string]interface{}) error {
										if _, ok := m["name"]; !ok {
											t.Errorf("machine.begin() nissing name %v", m)
											return fmt.Errorf("node.cascade() incorrect data have %v want %v", m, "name field")
										}
										return nil
									}).
										Terminate("terminus_id", "terminus", false, func(list []map[string]interface{}) error {
											for i, packet := range list {
												if !reflect.DeepEqual(packet, testList1[i]) {
													t.Errorf("machine.begin() incorrect data have %v want %v", packet, testList1[i])
												}
											}
											out <- list
											return fmt.Errorf("error everything")
										}),
								).
								ThenRight(
									NewVertex("node_id", "node", false, func(m map[string]interface{}) error {
										t.Errorf("no errors expected")
										return nil
									}).
										Terminate("terminus_id", "terminus", false, func(list []map[string]interface{}) error {
											t.Errorf("no errors expected")
											return nil
										}),
								),
						).
						RouteRight(
							NewRouter("route_id", "route", false, RouterError{}.Handler).
								TerminateLeft("terminus_id", "terminus", false, func(list []map[string]interface{}) error {
									t.Errorf("no errors expected")
									return nil
								}).
								TerminateRight("terminus_id", "terminus", false, func(list []map[string]interface{}) error {
									t.Errorf("no errors expected")
									return nil
								}),
						),
				),
			),
		).Build(func(s1, s2 string, p []*Packet) {})

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

func Test_New_FIFO(t *testing.T) {
	t.Run("Test_New", func(t *testing.T) {
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
		}).Then(
			NewVertex("node_id1", "node1", true, func(m map[string]interface{}) error {
				if _, ok := m["name"]; !ok {
					t.Errorf("machine.begin() nissing name %v", m)
					return fmt.Errorf("node.cascade() incorrect data have %v want %v", m, "name field")
				}
				return nil
			}).Then(
				NewVertex("node_id2", "node2", true, func(m map[string]interface{}) error {
					if _, ok := m["name"]; !ok {
						t.Errorf("machine.begin() nissing name %v", m)
						return fmt.Errorf("node.cascade() incorrect data have %v want %v", m, "name field")
					}
					return nil
				}).Route(
					NewRouter("route_id", "route", true, RouterError{}.Handler).
						RouteLeft(
							NewRouter("route_id", "route", true, RouterError{}.Handler).
								ThenLeft(
									NewVertex("node_id3", "node3", true, func(m map[string]interface{}) error {
										if _, ok := m["name"]; !ok {
											t.Errorf("machine.begin() nissing name %v", m)
											return fmt.Errorf("node.cascade() incorrect data have %v want %v", m, "name field")
										}
										return nil
									}).
										Terminate("terminus_id", "terminus", true, func(list []map[string]interface{}) error {
											for i, packet := range list {
												if !reflect.DeepEqual(packet, testList1[i]) {
													t.Errorf("machine.begin() incorrect data have %v want %v", packet, testList1[i])
												}
											}
											out <- list
											return fmt.Errorf("error everything")
										}),
								).
								ThenRight(
									NewVertex("node_id", "node", true, func(m map[string]interface{}) error {
										t.Errorf("no errors expected")
										return nil
									}).
										Terminate("terminus_id", "terminus", true, func(list []map[string]interface{}) error {
											t.Errorf("no errors expected")
											return nil
										}),
								),
						).
						RouteRight(
							NewRouter("route_id", "route", true, RouterError{}.Handler).
								TerminateLeft("terminus_id", "terminus", true, func(list []map[string]interface{}) error {
									t.Errorf("no errors expected")
									return nil
								}).
								TerminateRight("terminus_id", "terminus", true, func(list []map[string]interface{}) error {
									t.Errorf("no errors expected")
									return nil
								}),
						),
				),
			),
		).Build(func(s1, s2 string, p []*Packet) {})

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

func Test_New_Router(t *testing.T) {
	t.Run("Test_New_Router", func(t *testing.T) {
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
		}).Route(
			NewRouter("route_id", "route", false, RouterError{}.Handler).
				RouteLeft(
					NewRouter("route_id", "route", false, RouterError{}.Handler).
						ThenLeft(
							NewVertex("node_id3", "node3", false, func(m map[string]interface{}) error {
								if _, ok := m["name"]; !ok {
									t.Errorf("machine.begin() nissing name %v", m)
									return fmt.Errorf("node.cascade() incorrect data have %v want %v", m, "name field")
								}
								return nil
							}).
								Terminate("terminus_id", "terminus", false, func(list []map[string]interface{}) error {
									for i, packet := range list {
										if !reflect.DeepEqual(packet, testList1[i]) {
											t.Errorf("machine.begin() incorrect data have %v want %v", packet, testList1[i])
										}
									}
									out <- list
									return fmt.Errorf("error everything")
								}),
						).
						ThenRight(
							NewVertex("node_id", "node", false, func(m map[string]interface{}) error {
								t.Errorf("no errors expected")
								return nil
							}).
								Terminate("terminus_id", "terminus", false, func(list []map[string]interface{}) error {
									t.Errorf("no errors expected")
									return nil
								}),
						),
				).
				RouteRight(
					NewRouter("route_id", "route", false, RouterError{}.Handler).
						TerminateLeft("terminus_id", "terminus", false, func(list []map[string]interface{}) error {
							t.Errorf("no errors expected")
							return nil
						}).
						TerminateRight("terminus_id", "terminus", false, func(list []map[string]interface{}) error {
							t.Errorf("no errors expected")
							return nil
						}),
				),
		).Build(func(s1, s2 string, p []*Packet) {})

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

func Test_New_Empty_Payload(t *testing.T) {
	t.Run("Test_New_Termination", func(t *testing.T) {
		count := 100000

		m := New("machine_id", "machine", false, func(c context.Context) chan []map[string]interface{} {
			channel := make(chan []map[string]interface{})

			go func() {
				for i := 0; i < count; i++ {
					channel <- []map[string]interface{}{}
				}
			}()

			return channel
		}).
			Terminate("terminus_id", "terminus", false, func(list []map[string]interface{}) error {
				t.Errorf("no errors expected")
				return nil
			}).Build(func(s1, s2 string, p []*Packet) {})

		if err := m.Run(context.Background()); err != nil {
			t.Error(err)
		}
	})
}

func Test_New_Termination(t *testing.T) {
	t.Run("Test_New_Termination", func(t *testing.T) {
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
		}).
			Terminate("terminus_id", "terminus", false, func(list []map[string]interface{}) error {
				for i, packet := range list {
					if !reflect.DeepEqual(packet, testList1[i]) {
						t.Errorf("machine.begin() incorrect data have %v want %v", packet, testList1[i])
					}
				}
				out <- list
				return fmt.Errorf("error everything")
			}).Build(func(s1, s2 string, p []*Packet) {})

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

func Test_New_Cancellation(t *testing.T) {
	t.Run("Test_New", func(t *testing.T) {
		count := 100000
		out := make(chan []map[string]interface{})

		router := NewRouter("route_id", "route", false, RouterError{}.Handler).
			TerminateLeft("terminus_id", "terminus", false, func(list []map[string]interface{}) error {
				t.Errorf("no errors expected")
				return nil
			}).
			TerminateRight("terminus_id", "terminus", false, func(list []map[string]interface{}) error {
				t.Errorf("no errors expected")
				return nil
			})

		m := New("machine_id", "machine", false, func(c context.Context) chan []map[string]interface{} {
			channel := make(chan []map[string]interface{})

			go func() {
				for i := 0; i < count; i++ {
					channel <- testList1
				}
			}()

			return channel
		}).Then(
			NewVertex("node_id1", "node1", false, func(m map[string]interface{}) error {
				if _, ok := m["name"]; !ok {
					t.Errorf("machine.begin() nissing name %v", m)
					return fmt.Errorf("node.cascade() incorrect data have %v want %v", m, "name field")
				}
				return nil
			}).Then(
				NewVertex("node_id2", "node2", false, func(m map[string]interface{}) error {
					if _, ok := m["name"]; !ok {
						t.Errorf("machine.begin() nissing name %v", m)
						return fmt.Errorf("node.cascade() incorrect data have %v want %v", m, "name field")
					}
					return nil
				}).Route(
					NewRouter("route_id", "route", false, RouterError{}.Handler).
						RouteLeft(
							NewRouter("route_id", "route", false, RouterError{}.Handler).
								ThenLeft(
									NewVertex("node_id3", "node3", false, func(m map[string]interface{}) error {
										if _, ok := m["name"]; !ok {
											t.Errorf("machine.begin() nissing name %v", m)
											return fmt.Errorf("node.cascade() incorrect data have %v want %v", m, "name field")
										}
										return nil
									}).
										Terminate("terminus_id", "terminus", false, func(list []map[string]interface{}) error {
											for i, packet := range list {
												if !reflect.DeepEqual(packet, testList1[i]) {
													t.Errorf("machine.begin() incorrect data have %v want %v", packet, testList1[i])
												}
											}
											out <- list
											return fmt.Errorf("error everything")
										}),
								).
								ThenRight(
									NewVertex("node_id", "node", false, func(m map[string]interface{}) error {
										t.Errorf("no errors expected")
										return nil
									}).
										Route(router),
								),
						).
						RouteRight(router),
				),
			),
		).Build(func(s1, s2 string, p []*Packet) {})

		ctx, cancel := context.WithCancel(context.Background())

		if err := m.Run(ctx); err != nil {
			t.Error(err)
		}

		x := map[string][]*Packet{
			"node_id1": testPayload,
		}

		go func() {
			for {
				m.Inject(x)
			}
		}()

		<-time.After(time.Second / 3)

		cancel()

		<-time.After(time.Second)
	})
}

func Test_New_Missing_Termination(t *testing.T) {
	t.Run("Test_New", func(t *testing.T) {
		router := NewRouter("route_id", "route", false, RouterError{}.Handler).
			TerminateRight("terminus_id", "terminus", false, func(list []map[string]interface{}) error {
				t.Errorf("no errors expected")
				return nil
			})

		m := New("machine_id", "machine", false, func(c context.Context) chan []map[string]interface{} {
			channel := make(chan []map[string]interface{})
			return channel
		}).Then(
			NewVertex("node_id1", "node1", false, func(m map[string]interface{}) error {
				if _, ok := m["name"]; !ok {
					t.Errorf("machine.begin() nissing name %v", m)
					return fmt.Errorf("node.cascade() incorrect data have %v want %v", m, "name field")
				}
				return nil
			}).Then(
				NewVertex("node_id2", "node2", false, func(m map[string]interface{}) error {
					if _, ok := m["name"]; !ok {
						t.Errorf("machine.begin() nissing name %v", m)
						return fmt.Errorf("node.cascade() incorrect data have %v want %v", m, "name field")
					}
					return nil
				}).Route(
					NewRouter("route_id", "route", false, RouterError{}.Handler).
						RouteLeft(
							NewRouter("route_id", "route", false, RouterError{}.Handler).
								ThenLeft(
									NewVertex("node_id3", "node3", false, func(m map[string]interface{}) error {
										if _, ok := m["name"]; !ok {
											t.Errorf("machine.begin() nissing name %v", m)
											return fmt.Errorf("node.cascade() incorrect data have %v want %v", m, "name field")
										}
										return nil
									}),
								).
								ThenRight(
									NewVertex("node_id", "node", false, func(m map[string]interface{}) error {
										t.Errorf("no errors expected")
										return nil
									}).
										Route(router),
								),
						),
				),
			),
		).Build(func(s1, s2 string, p []*Packet) {})

		if err := m.Run(context.Background()); err == nil {
			t.Errorf("did not find errors")
		}

		m2 := New("machine_id", "machine", false, func(c context.Context) chan []map[string]interface{} {
			channel := make(chan []map[string]interface{})
			return channel
		}).Build(func(s1, s2 string, p []*Packet) {})

		if m2.ID() != "machine_id" {
			t.Errorf("incorrect id have %s want %s", m2.ID(), "machine_id")
		}

		if err := m2.Run(context.Background()); err == nil {
			t.Errorf("did not find errors")
		}

		m3 := New("machine_id", "machine", false, func(c context.Context) chan []map[string]interface{} {
			channel := make(chan []map[string]interface{})
			return channel
		}).Then(
			NewVertex("node_id1", "node1", false, func(m map[string]interface{}) error {
				if _, ok := m["name"]; !ok {
					t.Errorf("machine.begin() nissing name %v", m)
					return fmt.Errorf("node.cascade() incorrect data have %v want %v", m, "name field")
				}
				return nil
			}),
		).Build(func(s1, s2 string, p []*Packet) {})

		if err := m3.Run(context.Background()); err == nil {
			t.Errorf("did not find errors")
		}
	})
}

func Test_New_Duplication(t *testing.T) {
	t.Run("Test_New", func(t *testing.T) {
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
		}).Route(
			NewRouter("route_id", "route", false, RouterDuplicate{}.Handler).
				TerminateLeft("terminus_id", "terminus", false, func(list []map[string]interface{}) error {
					for i, packet := range list {
						if !reflect.DeepEqual(packet, testList1[i]) {
							t.Errorf("machine.begin() incorrect data have %v want %v", packet, testList1[i])
						}
					}
					out <- list
					return nil
				}).
				TerminateRight("terminus_id", "terminus", false, func(list []map[string]interface{}) error {
					for i, packet := range list {
						if !reflect.DeepEqual(packet, testList1[i]) {
							t.Errorf("machine.begin() incorrect data have %v want %v", packet, testList1[i])
						}
					}
					out <- list
					return nil
				}),
		).Build(func(s1, s2 string, p []*Packet) {})

		if err := m.Run(context.Background()); err != nil {
			t.Errorf("did not find errors")
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

func Test_New_Rule(t *testing.T) {
	t.Run("Test_New", func(t *testing.T) {
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
		}).Route(
			NewRouter("route_id", "route", false, RouterRule(func(m map[string]interface{}) bool { return true }).Handler).
				TerminateLeft("terminus_id", "terminus", false, func(list []map[string]interface{}) error {
					for i, packet := range list {
						if !reflect.DeepEqual(packet, testList1[i]) {
							t.Errorf("machine.begin() incorrect data have %v want %v", packet, testList1[i])
						}
					}
					out <- list
					return nil
				}).
				TerminateRight("terminus_id", "terminus", false, func(list []map[string]interface{}) error {
					t.Errorf("no errors expected")
					return nil
				}),
		).Build(func(s1, s2 string, p []*Packet) {})

		if err := m.Run(context.Background()); err != nil {
			t.Errorf("did not find errors")
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

func Test_New_Reuse_Node(t *testing.T) {
	t.Run("Test_New", func(t *testing.T) {
		count := 100000
		out := make(chan []map[string]interface{})

		node := NewVertex("node_id1", "node1", false, func(m map[string]interface{}) error {
			if _, ok := m["name"]; !ok {
				t.Errorf("machine.begin() nissing name %v", m)
				return fmt.Errorf("node.cascade() incorrect data have %v want %v", m, "name field")
			}
			return fmt.Errorf("fail everything")
		}).
			Terminate("terminus_id", "terminus", false, func(list []map[string]interface{}) error {
				for i, packet := range list {
					if !reflect.DeepEqual(packet, testList1[i]) {
						t.Errorf("machine.begin() incorrect data have %v want %v", packet, testList1[i])
					}
				}
				out <- list
				return nil
			})

		m := New("machine_id", "machine", false, func(c context.Context) chan []map[string]interface{} {
			channel := make(chan []map[string]interface{})

			go func() {
				for i := 0; i < count; i++ {
					channel <- testList1
				}
			}()

			return channel
		}).
			Then(node).
			Build(func(s1, s2 string, p []*Packet) {})

		if err := m.Run(context.Background()); err != nil {
			t.Errorf("did not find errors")
		}

		m2 := New("machine_id", "machine", false, func(c context.Context) chan []map[string]interface{} {
			channel := make(chan []map[string]interface{})

			go func() {
				for i := 0; i < count; i++ {
					channel <- testList1
				}
			}()

			return channel
		}).
			Then(node).Build()

		if err := m2.Run(context.Background()); err != nil {
			t.Errorf("did not find errors")
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

func Test_New_RouterError_Error(t *testing.T) {
	t.Run("Test_New", func(t *testing.T) {
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
		}).Then(
			NewVertex("node_id1", "node1", false, func(m map[string]interface{}) error {
				if _, ok := m["name"]; !ok {
					t.Errorf("machine.begin() nissing name %v", m)
					return fmt.Errorf("node.cascade() incorrect data have %v want %v", m, "name field")
				}
				return fmt.Errorf("fail everything")
			}).Route(
				NewRouter("route_id", "route", false, RouterError{}.Handler).
					TerminateLeft("terminus_id", "terminus", false, func(list []map[string]interface{}) error {
						t.Errorf("no errors expected")
						return nil
					}).
					TerminateRight("terminus_id", "terminus", false, func(list []map[string]interface{}) error {
						for i, packet := range list {
							if !reflect.DeepEqual(packet, testList1[i]) {
								t.Errorf("machine.begin() incorrect data have %v want %v", packet, testList1[i])
							}
						}
						out <- list
						return nil
					}),
			),
		).Build(func(s1, s2 string, p []*Packet) {})

		if err := m.Run(context.Background()); err != nil {
			t.Errorf("did not find errors")
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

func Test_New_Rule_False(t *testing.T) {
	t.Run("Test_New", func(t *testing.T) {
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
		}).Route(
			NewRouter("route_id", "route", false, RouterRule(func(m map[string]interface{}) bool { return false }).Handler).
				TerminateLeft("terminus_id", "terminus", false, func(list []map[string]interface{}) error {
					t.Errorf("no errors expected")
					return nil
				}).
				TerminateRight("terminus_id", "terminus", false, func(list []map[string]interface{}) error {
					for i, packet := range list {
						if !reflect.DeepEqual(packet, testList1[i]) {
							t.Errorf("machine.begin() incorrect data have %v want %v", packet, testList1[i])
						}
					}
					out <- list
					return nil
				}),
		).Build(func(s1, s2 string, p []*Packet) {})

		if err := m.Run(context.Background()); err != nil {
			t.Errorf("did not find errors")
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

func Test_New_Rule_Left_Error(t *testing.T) {
	t.Run("Test_New_Rule_Left_Error", func(t *testing.T) {
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
		}).Route(
			NewRouter("route_id", "route", false, RouterRule(func(m map[string]interface{}) bool { return false }).Handler).
				ThenLeft(
					NewVertex("node_id1", "node1", false, func(m map[string]interface{}) error {
						if _, ok := m["name"]; !ok {
							t.Errorf("machine.begin() nissing name %v", m)
							return fmt.Errorf("node.cascade() incorrect data have %v want %v", m, "name field")
						}
						return fmt.Errorf("fail everything")
					}),
				).
				TerminateRight("terminus_id", "terminus", false, func(list []map[string]interface{}) error {
					for i, packet := range list {
						if !reflect.DeepEqual(packet, testList1[i]) {
							t.Errorf("machine.begin() incorrect data have %v want %v", packet, testList1[i])
						}
					}
					out <- list
					return nil
				}),
		).Build(func(s1, s2 string, p []*Packet) {})

		if err := m.Run(context.Background()); err == nil {
			t.Errorf("did not find errors")
		}
	})
}

func Test_New_Rule_Right_Error(t *testing.T) {
	t.Run("Test_New_Rule_Right_Error", func(t *testing.T) {
		count := 100000

		m := New("machine_id", "machine", false, func(c context.Context) chan []map[string]interface{} {
			channel := make(chan []map[string]interface{})

			go func() {
				for i := 0; i < count; i++ {
					channel <- testList1
				}
			}()

			return channel
		}).Route(
			NewRouter("route_id", "route", false, RouterRule(func(m map[string]interface{}) bool { return false }).Handler).
				TerminateLeft("terminus_id", "terminus", false, func(list []map[string]interface{}) error {
					t.Errorf("no errors expected")
					return nil
				}).
				ThenRight(
					NewVertex("node_id1", "node1", false, func(m map[string]interface{}) error {
						if _, ok := m["name"]; !ok {
							t.Errorf("machine.begin() nissing name %v", m)
							return fmt.Errorf("node.cascade() incorrect data have %v want %v", m, "name field")
						}
						return fmt.Errorf("fail everything")
					}),
				),
		).Build(func(s1, s2 string, p []*Packet) {})

		if err := m.Run(context.Background()); err == nil {
			t.Errorf("did not find errors")
		}
	})
}
