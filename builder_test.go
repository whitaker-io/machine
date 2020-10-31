// Copyright © 2020 Jonathan Whitaker <github@whitaker.io>.
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package machine

import (
	"context"
	"fmt"
	"reflect"
	"testing"
	"time"
)

var testList = []map[string]interface{}{
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

var bufferSize = 0

func Benchmark_Test_New(b *testing.B) {
	out := make(chan []map[string]interface{})
	channel := make(chan []map[string]interface{})
	m := New("machine_id", func(c context.Context) chan []map[string]interface{} {
		return channel
	}).Then(
		NewVertex("node_id1", func(m map[string]interface{}) error {
			if _, ok := m["name"]; !ok {
				b.Errorf("packet missing name %v", m)
				return fmt.Errorf("incorrect data have %v want %v", m, "name field")
			}
			return nil
		}).Then(
			NewVertex("node_id2", func(m map[string]interface{}) error {
				if _, ok := m["name"]; !ok {
					b.Errorf("packet missing name %v", m)
					return fmt.Errorf("incorrect data have %v want %v", m, "name field")
				}
				return nil
			}).Route(
				NewRouter("route_id", RouterError).
					RouteLeft(
						NewRouter("route_id", RouterError).
							ThenLeft(
								NewVertex("node_id3", func(m map[string]interface{}) error {
									if _, ok := m["name"]; !ok {
										b.Errorf("packet missing name %v", m)
										return fmt.Errorf("incorrect data have %v want %v", m, "name field")
									}
									return nil
								}).
									Terminate(NewTermination("terminus_id", func(list []map[string]interface{}) error {
										out <- list
										return nil
									})),
							).
							ThenRight(
								NewVertex("node_id", func(m map[string]interface{}) error {
									b.Errorf("no errors expected")
									return nil
								}).
									Terminate(NewTermination("terminus_id", func(list []map[string]interface{}) error {
										b.Errorf("no errors expected")
										return nil
									})),
							),
					).
					RouteRight(
						NewRouter("route_id", RouterError).
							TerminateLeft(NewTermination("terminus_id", func(list []map[string]interface{}) error {
								b.Errorf("no errors expected")
								return nil
							})).
							TerminateRight(NewTermination("terminus_id", func(list []map[string]interface{}) error {
								b.Errorf("no errors expected")
								return nil
							})),
					),
			),
		),
	)

	if err := m.Run(context.Background()); err != nil {
		b.Error(err)
	}

	for n := 0; n < b.N; n++ {
		go func() {
			channel <- testList
		}()

		list := <-out

		if len(list) != len(testList) {
			b.Errorf("incorrect data have %v want %v", list, testList)
		}
	}
}

func Test_New(t *testing.T) {
	count := 100000
	out := make(chan []map[string]interface{})
	t.Run("Test_New", func(t *testing.T) {

		m := New("machine_id", func(c context.Context) chan []map[string]interface{} {
			channel := make(chan []map[string]interface{})

			go func() {
				for i := 0; i < count; i++ {
					channel <- testList
				}
			}()

			return channel
		}).Then(
			NewVertex("node_id1", func(m map[string]interface{}) error {
				if _, ok := m["name"]; !ok {
					t.Errorf("packet missing name %v", m)
					return fmt.Errorf("incorrect data have %v want %v", m, "name field")
				}
				return nil
			}).Then(
				NewVertex("node_id2", func(m map[string]interface{}) error {
					if _, ok := m["name"]; !ok {
						t.Errorf("packet missing name %v", m)
						return fmt.Errorf("incorrect data have %v want %v", m, "name field")
					}
					return nil
				}).Route(
					NewRouter("route_id", RouterError).
						RouteLeft(
							NewRouter("route_id", RouterError).
								ThenLeft(
									NewVertex("node_id3", func(m map[string]interface{}) error {
										if _, ok := m["name"]; !ok {
											t.Errorf("packet missing name %v", m)
											return fmt.Errorf("incorrect data have %v want %v", m, "name field")
										}
										return nil
									}).
										Terminate(NewTermination("terminus_id", func(list []map[string]interface{}) error {
											for i, packet := range list {
												if !reflect.DeepEqual(packet, testList[i]) {
													t.Errorf("incorrect data have %v want %v", packet, testList[i])
												}
											}
											out <- list
											return fmt.Errorf("error everything")
										})),
								).
								ThenRight(
									NewVertex("node_id", func(m map[string]interface{}) error {
										t.Errorf("no errors expected")
										return nil
									}).
										Terminate(NewTermination("terminus_id", func(list []map[string]interface{}) error {
											t.Errorf("no errors expected")
											return nil
										})),
								),
						).
						RouteRight(
							NewRouter("route_id", RouterError).
								TerminateLeft(NewTermination("terminus_id", func(list []map[string]interface{}) error {
									t.Errorf("no errors expected")
									return nil
								})).
								TerminateRight(NewTermination("terminus_id", func(list []map[string]interface{}) error {
									t.Errorf("no errors expected")
									return nil
								})),
						),
				),
			),
		)

		if err := m.Run(context.Background(), func(s1, s2, s3 string, p []*Packet) {}); err != nil {
			t.Error(err)
		}

		for i := 0; i < count; i++ {
			list1 := <-out
			for i, packet := range list1 {
				if !reflect.DeepEqual(packet, testList[i]) {
					t.Errorf("incorrect data have %v want %v", packet, testList[i])
				}
			}
		}
	})
}

func Test_New_FIFO(t *testing.T) {
	t.Run("Test_New_FIFO", func(t *testing.T) {
		count := 10000
		out := make(chan []map[string]interface{})

		term := NewTermination("terminus_id", func(list []map[string]interface{}) error {
			t.Errorf("no errors expected")
			return nil
		})

		m := New("machine_id", func(c context.Context) chan []map[string]interface{} {
			channel := make(chan []map[string]interface{})

			go func() {
				for i := 0; i < count; i++ {
					channel <- testList
				}
			}()

			return channel
		}, &Option{FIFO: boolP(true)}).Then(
			NewVertex("node_id1", func(m map[string]interface{}) error {
				if _, ok := m["name"]; !ok {
					t.Errorf("packet missing name %v", m)
					return fmt.Errorf("incorrect data have %v want %v", m, "name field")
				}
				return nil
			}).Then(
				NewVertex("node_id2", func(m map[string]interface{}) error {
					if _, ok := m["name"]; !ok {
						t.Errorf("packet missing name %v", m)
						return fmt.Errorf("incorrect data have %v want %v", m, "name field")
					}
					return nil
				}).Route(
					NewRouter("route_id", RouterError).
						RouteLeft(
							NewRouter("route_id", RouterError).
								ThenLeft(
									NewVertex("node_id3", func(m map[string]interface{}) error {
										if _, ok := m["name"]; !ok {
											t.Errorf("packet missing name %v", m)
											return fmt.Errorf("incorrect data have %v want %v", m, "name field")
										}
										return nil
									}).
										Terminate(NewTermination("terminus_id", func(list []map[string]interface{}) error {
											for i, packet := range list {
												if !reflect.DeepEqual(packet, testList[i]) {
													t.Errorf("incorrect data have %v want %v", packet, testList[i])
												}
											}
											out <- list
											return fmt.Errorf("error everything")
										})),
								).
								ThenRight(
									NewVertex("node_id", func(m map[string]interface{}) error {
										t.Errorf("no errors expected")
										return nil
									}).
										Terminate(term),
								),
						).
						RouteRight(
							NewRouter("route_id", RouterError).
								TerminateLeft(term).
								TerminateRight(term),
						),
				),
			),
		)

		if err := m.Run(context.Background()); err != nil {
			t.Error(err)
		}

		for i := 0; i < count; i++ {
			list1 := <-out
			for i, packet := range list1 {
				if !reflect.DeepEqual(packet, testList[i]) {
					t.Errorf("incorrect data have %v want %v", packet, testList[i])
				}
			}
		}
	})
}

func Test_New_All_Options(t *testing.T) {
	t.Run("Test_New_FIFO", func(t *testing.T) {
		count := 10000
		out := make(chan []map[string]interface{})

		term := NewTermination("terminus_id", func(list []map[string]interface{}) error {
			t.Errorf("no errors expected")
			return nil
		})

		m := New("machine_id", func(c context.Context) chan []map[string]interface{} {
			channel := make(chan []map[string]interface{})

			go func() {
				for i := 0; i < count; i++ {
					channel <- testList
				}
			}()

			return channel
		}, &Option{FIFO: boolP(true)}, &Option{Idempotent: boolP(true)}, &Option{BufferSize: intP(10)}).Then(
			NewVertex("node_id1", func(m map[string]interface{}) error {
				if _, ok := m["name"]; !ok {
					t.Errorf("packet missing name %v", m)
					return fmt.Errorf("incorrect data have %v want %v", m, "name field")
				}
				return nil
			}).Then(
				NewVertex("node_id2", func(m map[string]interface{}) error {
					if _, ok := m["name"]; !ok {
						t.Errorf("packet missing name %v", m)
						return fmt.Errorf("incorrect data have %v want %v", m, "name field")
					}
					return nil
				}).Route(
					NewRouter("route_id", RouterError).
						RouteLeft(
							NewRouter("route_id", RouterError).
								ThenLeft(
									NewVertex("node_id3", func(m map[string]interface{}) error {
										if _, ok := m["name"]; !ok {
											t.Errorf("packet missing name %v", m)
											return fmt.Errorf("incorrect data have %v want %v", m, "name field")
										}
										return nil
									}).
										Terminate(NewTermination("terminus_id", func(list []map[string]interface{}) error {
											for i, packet := range list {
												if !reflect.DeepEqual(packet, testList[i]) {
													t.Errorf("incorrect data have %v want %v", packet, testList[i])
												}
											}
											out <- list
											return fmt.Errorf("error everything")
										})),
								).
								ThenRight(
									NewVertex("node_id", func(m map[string]interface{}) error {
										t.Errorf("no errors expected")
										return nil
									}).
										Terminate(term),
								),
						).
						RouteRight(
							NewRouter("route_id", RouterError).
								TerminateLeft(term).
								TerminateRight(term),
						),
				),
			),
		)

		if err := m.Run(context.Background()); err != nil {
			t.Error(err)
		}

		for i := 0; i < count; i++ {
			list1 := <-out
			for i, packet := range list1 {
				if !reflect.DeepEqual(packet, testList[i]) {
					t.Errorf("incorrect data have %v want %v", packet, testList[i])
				}
			}
		}
	})
}

func Test_New_Router(t *testing.T) {
	t.Run("Test_New_Router", func(t *testing.T) {
		count := 10000
		out := make(chan []map[string]interface{})

		m := New("machine_id", func(c context.Context) chan []map[string]interface{} {
			channel := make(chan []map[string]interface{})

			go func() {
				for i := 0; i < count; i++ {
					channel <- testList
				}
			}()

			return channel
		}).Route(
			NewRouter("route_id", RouterError).
				RouteLeft(
					NewRouter("route_id", RouterError).
						ThenLeft(
							NewVertex("node_id3", func(m map[string]interface{}) error {
								if _, ok := m["name"]; !ok {
									t.Errorf("packet missing name %v", m)
									return fmt.Errorf("incorrect data have %v want %v", m, "name field")
								}
								return nil
							}).
								Terminate(NewTermination("terminus_id", func(list []map[string]interface{}) error {
									for i, packet := range list {
										if !reflect.DeepEqual(packet, testList[i]) {
											t.Errorf("incorrect data have %v want %v", packet, testList[i])
										}
									}
									out <- list
									return fmt.Errorf("error everything")
								})),
						).
						ThenRight(
							NewVertex("node_id", func(m map[string]interface{}) error {
								t.Errorf("no errors expected")
								return nil
							}).
								Terminate(NewTermination("terminus_id", func(list []map[string]interface{}) error {
									t.Errorf("no errors expected")
									return nil
								})),
						),
				).
				RouteRight(
					NewRouter("route_id", RouterError).
						TerminateLeft(NewTermination("terminus_id", func(list []map[string]interface{}) error {
							t.Errorf("no errors expected")
							return nil
						})).
						TerminateRight(NewTermination("terminus_id", func(list []map[string]interface{}) error {
							t.Errorf("no errors expected")
							return nil
						})),
				),
		)

		if err := m.Run(context.Background()); err != nil {
			t.Error(err)
		}

		for i := 0; i < count; i++ {
			list1 := <-out
			for i, packet := range list1 {
				if !reflect.DeepEqual(packet, testList[i]) {
					t.Errorf("incorrect data have %v want %v", packet, testList[i])
				}
			}
		}
	})
}

func Test_New_Empty_Payload(t *testing.T) {
	t.Run("Test_New_Empty_Payload", func(t *testing.T) {
		count := 10000

		m := New("machine_id", func(c context.Context) chan []map[string]interface{} {
			channel := make(chan []map[string]interface{})

			go func() {
				for i := 0; i < count; i++ {
					channel <- []map[string]interface{}{}
				}
			}()

			return channel
		}).
			Terminate(NewTermination("terminus_id", func(list []map[string]interface{}) error {
				t.Errorf("no errors expected")
				return nil
			}))

		if err := m.Run(context.Background()); err != nil {
			t.Error(err)
		}
	})
}

func Test_New_Termination(t *testing.T) {
	t.Run("Test_New_Termination", func(t *testing.T) {
		count := 10000
		out := make(chan []map[string]interface{})

		m := New("machine_id", func(c context.Context) chan []map[string]interface{} {
			channel := make(chan []map[string]interface{})

			go func() {
				for i := 0; i < count; i++ {
					channel <- testList
				}
			}()

			return channel
		}).
			Terminate(NewTermination("terminus_id", func(list []map[string]interface{}) error {
				for i, packet := range list {
					if !reflect.DeepEqual(packet, testList[i]) {
						t.Errorf("incorrect data have %v want %v", packet, testList[i])
					}
				}
				out <- list
				return fmt.Errorf("error everything")
			}))

		if err := m.Run(context.Background()); err != nil {
			t.Error(err)
		}

		for i := 0; i < count; i++ {
			list1 := <-out
			for i, packet := range list1 {
				if !reflect.DeepEqual(packet, testList[i]) {
					t.Errorf("incorrect data have %v want %v", packet, testList[i])
				}
			}
		}
	})
}

func Test_New_Cancellation(t *testing.T) {
	t.Run("Test_New_Cancellation", func(t *testing.T) {
		count := 10000
		out := make(chan []map[string]interface{})

		router := NewRouter("route_id", RouterError).
			TerminateLeft(NewTermination("terminus_id", func(list []map[string]interface{}) error {
				t.Errorf("no errors expected")
				return nil
			})).
			TerminateRight(NewTermination("terminus_id", func(list []map[string]interface{}) error {
				t.Errorf("no errors expected")
				return nil
			}))

		m := New("machine_id", func(c context.Context) chan []map[string]interface{} {
			channel := make(chan []map[string]interface{})

			go func() {
				for i := 0; i < count; i++ {
					channel <- testList
				}
			}()

			return channel
		}).Then(
			NewVertex("node_id1", func(m map[string]interface{}) error {
				if _, ok := m["name"]; !ok {
					t.Errorf("packet missing name %v", m)
					return fmt.Errorf("incorrect data have %v want %v", m, "name field")
				}
				return nil
			}).Then(
				NewVertex("node_id2", func(m map[string]interface{}) error {
					if _, ok := m["name"]; !ok {
						t.Errorf("packet missing name %v", m)
						return fmt.Errorf("incorrect data have %v want %v", m, "name field")
					}
					return nil
				}).Route(
					NewRouter("route_id", RouterError).
						RouteLeft(
							NewRouter("route_id", RouterError).
								ThenLeft(
									NewVertex("node_id3", func(m map[string]interface{}) error {
										if _, ok := m["name"]; !ok {
											t.Errorf("packet missing name %v", m)
											return fmt.Errorf("incorrect data have %v want %v", m, "name field")
										}
										return nil
									}).
										Terminate(NewTermination("terminus_id", func(list []map[string]interface{}) error {
											for i, packet := range list {
												if !reflect.DeepEqual(packet, testList[i]) {
													t.Errorf("incorrect data have %v want %v", packet, testList[i])
												}
											}
											out <- list
											return fmt.Errorf("error everything")
										})),
								).
								ThenRight(
									NewVertex("node_id", func(m map[string]interface{}) error {
										t.Errorf("no errors expected")
										return nil
									}).
										Route(router),
								),
						).
						RouteRight(router),
				),
			),
		)

		ctx, cancel := context.WithCancel(context.Background())

		if err := m.Run(ctx); err != nil {
			t.Error(err)
		}

		x := map[string][]*Packet{
			"node_id1":   testPayload,
			"machine_id": testPayload,
		}

		go func() {
			for i := 0; i < count; i++ {
				m.Inject(ctx, x)
			}
		}()

		<-time.After(time.Second / 3)

		cancel()

		<-time.After(time.Second)
	})
}

func Test_New_Missing_Termination(t *testing.T) {
	t.Run("Test_New_Missing_Termination", func(t *testing.T) {
		router := NewRouter("route_id", RouterError).
			TerminateRight(NewTermination("terminus_id", func(list []map[string]interface{}) error {
				t.Errorf("no errors expected")
				return nil
			}))

		m := New("machine_id", func(c context.Context) chan []map[string]interface{} {
			channel := make(chan []map[string]interface{})
			return channel
		}).Then(
			NewVertex("node_id1", func(m map[string]interface{}) error {
				if _, ok := m["name"]; !ok {
					t.Errorf("packet missing name %v", m)
					return fmt.Errorf("incorrect data have %v want %v", m, "name field")
				}
				return nil
			}).Then(
				NewVertex("node_id2", func(m map[string]interface{}) error {
					if _, ok := m["name"]; !ok {
						t.Errorf("packet missing name %v", m)
						return fmt.Errorf("incorrect data have %v want %v", m, "name field")
					}
					return nil
				}).Route(
					NewRouter("route_id", RouterError).
						RouteLeft(
							NewRouter("route_id", RouterError).
								ThenLeft(
									NewVertex("node_id3", func(m map[string]interface{}) error {
										if _, ok := m["name"]; !ok {
											t.Errorf("packet missing name %v", m)
											return fmt.Errorf("incorrect data have %v want %v", m, "name field")
										}
										return nil
									}),
								).
								ThenRight(
									NewVertex("node_id", func(m map[string]interface{}) error {
										t.Errorf("no errors expected")
										return nil
									}).
										Route(router),
								),
						),
				),
			),
		)

		if err := m.Run(context.Background(), func(s1, s2, s3 string, p []*Packet) {}); err == nil {
			t.Errorf("did not find errors")
		}

		m2 := New("machine_id", func(c context.Context) chan []map[string]interface{} {
			channel := make(chan []map[string]interface{})
			return channel
		})

		if m2.ID() != "machine_id" {
			t.Errorf("incorrect id have %s want %s", m2.ID(), "machine_id")
		}

		if err := m2.Run(context.Background(), func(s1, s2, s3 string, p []*Packet) {}); err == nil {
			t.Errorf("did not find errors")
		}

		m3 := New("machine_id", func(c context.Context) chan []map[string]interface{} {
			channel := make(chan []map[string]interface{})
			return channel
		}).Then(
			NewVertex("node_id1", func(m map[string]interface{}) error {
				if _, ok := m["name"]; !ok {
					t.Errorf("packet missing name %v", m)
					return fmt.Errorf("incorrect data have %v want %v", m, "name field")
				}
				return nil
			}),
		)

		if err := m3.Run(context.Background(), func(s1, s2, s3 string, p []*Packet) {}); err == nil {
			t.Errorf("did not find errors")
		}
	})
}

func Test_New_Duplication(t *testing.T) {
	t.Run("Test_New_Duplication", func(t *testing.T) {
		count := 10000
		out := make(chan []map[string]interface{})

		m := New("machine_id", func(c context.Context) chan []map[string]interface{} {
			channel := make(chan []map[string]interface{})

			go func() {
				for i := 0; i < count; i++ {
					channel <- testList
				}
			}()

			return channel
		}).Route(
			NewRouter("route_id", RouterDuplicate).
				TerminateLeft(NewTermination("terminus_id", func(list []map[string]interface{}) error {
					for i, packet := range list {
						if !reflect.DeepEqual(packet, testList[i]) {
							t.Errorf("incorrect data have %v want %v", packet, testList[i])
						}
					}
					out <- list
					return nil
				})).
				TerminateRight(NewTermination("terminus_id", func(list []map[string]interface{}) error {
					for i, packet := range list {
						if !reflect.DeepEqual(packet, testList[i]) {
							t.Errorf("incorrect data have %v want %v", packet, testList[i])
						}
					}
					out <- list
					return nil
				})),
		)

		if err := m.Run(context.Background()); err != nil {
			t.Errorf("did not find errors")
		}

		for i := 0; i < count*2; i++ {
			list1 := <-out
			for i, packet := range list1 {
				if !reflect.DeepEqual(packet, testList[i]) {
					t.Errorf("incorrect data have %v want %v", packet, testList[i])
				}
			}
		}
	})
}

func Test_New_Rule(t *testing.T) {
	t.Run("Test_New_Rule", func(t *testing.T) {
		count := 10000
		out := make(chan []map[string]interface{})

		m := New("machine_id", func(c context.Context) chan []map[string]interface{} {
			channel := make(chan []map[string]interface{})

			go func() {
				for i := 0; i < count; i++ {
					channel <- testList
				}
			}()

			return channel
		}).Route(
			NewRouter("route_id", RouterRule(func(m map[string]interface{}) bool { return true }).Handler).
				TerminateLeft(NewTermination("terminus_id", func(list []map[string]interface{}) error {
					for i, packet := range list {
						if !reflect.DeepEqual(packet, testList[i]) {
							t.Errorf("incorrect data have %v want %v", packet, testList[i])
						}
					}
					out <- list
					return nil
				})).
				TerminateRight(NewTermination("terminus_id", func(list []map[string]interface{}) error {
					t.Errorf("no errors expected")
					return nil
				})),
		)

		if err := m.Run(context.Background(), func(s1, s2, s3 string, p []*Packet) {}); err != nil {
			t.Errorf("did not find errors")
		}

		for i := 0; i < count; i++ {
			list1 := <-out
			for i, packet := range list1 {
				if !reflect.DeepEqual(packet, testList[i]) {
					t.Errorf("incorrect data have %v want %v", packet, testList[i])
				}
			}
		}
	})
}

func Test_New_Reuse_Node(t *testing.T) {
	t.Run("Test_New_Reuse_Node", func(t *testing.T) {
		count := 10000
		out := make(chan []map[string]interface{})

		node := NewVertex("node_id1", func(m map[string]interface{}) error {
			if _, ok := m["name"]; !ok {
				t.Errorf("packet missing name %v", m)
				return fmt.Errorf("incorrect data have %v want %v", m, "name field")
			}
			return fmt.Errorf("fail everything")
		}).
			Terminate(NewTermination("terminus_id", func(list []map[string]interface{}) error {
				for i, packet := range list {
					if !reflect.DeepEqual(packet, testList[i]) {
						t.Errorf("incorrect data have %v want %v", packet, testList[i])
					}
				}
				out <- list
				return nil
			}))

		m := New("machine_id", func(c context.Context) chan []map[string]interface{} {
			channel := make(chan []map[string]interface{})

			go func() {
				for i := 0; i < count; i++ {
					channel <- testList
				}
			}()

			return channel
		}).
			Then(node)

		if err := m.Run(context.Background(), func(s1, s2, s3 string, p []*Packet) {}); err != nil {
			t.Errorf("did not find errors")
		}

		m2 := New("machine_id", func(c context.Context) chan []map[string]interface{} {
			channel := make(chan []map[string]interface{})

			go func() {
				for i := 0; i < count; i++ {
					channel <- testList
				}
			}()

			return channel
		}).
			Then(node)

		if err := m2.Run(context.Background(), func(s1, s2, s3 string, p []*Packet) {}); err != nil {
			t.Errorf("did not find errors")
		}

		for i := 0; i < count*2; i++ {
			list1 := <-out
			for i, packet := range list1 {
				if !reflect.DeepEqual(packet, testList[i]) {
					t.Errorf("incorrect data have %v want %v", packet, testList[i])
				}
			}
		}
	})
}

func Test_New_RouterError_Error(t *testing.T) {
	t.Run("Test_New_RouterError_Error", func(t *testing.T) {
		count := 10000
		out := make(chan []map[string]interface{})

		m := New("machine_id", func(c context.Context) chan []map[string]interface{} {
			channel := make(chan []map[string]interface{})

			go func() {
				for i := 0; i < count; i++ {
					channel <- testList
				}
			}()

			return channel
		}).Then(
			NewVertex("node_id1", func(m map[string]interface{}) error {
				if _, ok := m["name"]; !ok {
					t.Errorf("packet missing name %v", m)
					return fmt.Errorf("incorrect data have %v want %v", m, "name field")
				}
				return fmt.Errorf("fail everything")
			}).Route(
				NewRouter("route_id", RouterError).
					TerminateLeft(NewTermination("terminus_id", func(list []map[string]interface{}) error {
						t.Errorf("no errors expected")
						return nil
					})).
					TerminateRight(NewTermination("terminus_id", func(list []map[string]interface{}) error {
						for i, packet := range list {
							if !reflect.DeepEqual(packet, testList[i]) {
								t.Errorf("incorrect data have %v want %v", packet, testList[i])
							}
						}
						out <- list
						return nil
					})),
			),
		)

		if err := m.Run(context.Background(), func(s1, s2, s3 string, p []*Packet) {}); err != nil {
			t.Errorf("did not find errors")
		}

		for i := 0; i < count; i++ {
			list1 := <-out
			for i, packet := range list1 {
				if !reflect.DeepEqual(packet, testList[i]) {
					t.Errorf("incorrect data have %v want %v", packet, testList[i])
				}
			}
		}
	})
}

func Test_New_Rule_False(t *testing.T) {
	t.Run("Test_New_Rule_False", func(t *testing.T) {
		count := 10000
		out := make(chan []map[string]interface{})

		m := New("machine_id", func(c context.Context) chan []map[string]interface{} {
			channel := make(chan []map[string]interface{})

			go func() {
				for i := 0; i < count; i++ {
					channel <- testList
				}
			}()

			return channel
		}).Route(
			NewRouter("route_id", RouterRule(func(m map[string]interface{}) bool { return false }).Handler).
				TerminateLeft(NewTermination("terminus_id", func(list []map[string]interface{}) error {
					t.Errorf("no errors expected")
					return nil
				})).
				TerminateRight(NewTermination("terminus_id", func(list []map[string]interface{}) error {
					for i, packet := range list {
						if !reflect.DeepEqual(packet, testList[i]) {
							t.Errorf("incorrect data have %v want %v", packet, testList[i])
						}
					}
					out <- list
					return nil
				})),
		)

		if err := m.Run(context.Background(), func(s1, s2, s3 string, p []*Packet) {}); err != nil {
			t.Errorf("did not find errors")
		}

		for i := 0; i < count; i++ {
			list1 := <-out
			for i, packet := range list1 {
				if !reflect.DeepEqual(packet, testList[i]) {
					t.Errorf("incorrect data have %v want %v", packet, testList[i])
				}
			}
		}
	})
}

func Test_New_Rule_Left_Error(t *testing.T) {
	t.Run("Test_New_Rule_Left_Error", func(t *testing.T) {
		count := 10000
		out := make(chan []map[string]interface{})

		m := New("machine_id", func(c context.Context) chan []map[string]interface{} {
			channel := make(chan []map[string]interface{})

			go func() {
				for i := 0; i < count; i++ {
					channel <- testList
				}
			}()

			return channel
		}).Route(
			NewRouter("route_id", RouterRule(func(m map[string]interface{}) bool { return false }).Handler).
				ThenLeft(
					NewVertex("node_id1", func(m map[string]interface{}) error {
						if _, ok := m["name"]; !ok {
							t.Errorf("packet missing name %v", m)
							return fmt.Errorf("incorrect data have %v want %v", m, "name field")
						}
						return fmt.Errorf("fail everything")
					}),
				).
				TerminateRight(NewTermination("terminus_id", func(list []map[string]interface{}) error {
					for i, packet := range list {
						if !reflect.DeepEqual(packet, testList[i]) {
							t.Errorf("incorrect data have %v want %v", packet, testList[i])
						}
					}
					out <- list
					return nil
				})),
		)

		if err := m.Run(context.Background(), func(s1, s2, s3 string, p []*Packet) {}); err == nil {
			t.Errorf("did not find errors")
		}
	})
}

func Test_New_Rule_Right_Error(t *testing.T) {
	t.Run("Test_New_Rule_Right_Error", func(t *testing.T) {
		count := 10000

		m := New("machine_id", func(c context.Context) chan []map[string]interface{} {
			channel := make(chan []map[string]interface{})

			go func() {
				for i := 0; i < count; i++ {
					channel <- testList
				}
			}()

			return channel
		}).Route(
			NewRouter("route_id", RouterRule(func(m map[string]interface{}) bool { return false }).Handler).
				TerminateLeft(NewTermination("terminus_id", func(list []map[string]interface{}) error {
					t.Errorf("no errors expected")
					return nil
				})).
				ThenRight(
					NewVertex("node_id1", func(m map[string]interface{}) error {
						if _, ok := m["name"]; !ok {
							t.Errorf("packet missing name %v", m)
							return fmt.Errorf("incorrect data have %v want %v", m, "name field")
						}
						return fmt.Errorf("fail everything")
					}),
				),
		)

		if err := m.Run(context.Background(), func(s1, s2, s3 string, p []*Packet) {}); err == nil {
			t.Errorf("did not find errors")
		}
	})
}
