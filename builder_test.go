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

var testList = []Data{
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
	{
		"name":  "data8",
		"value": 8,
	},
	{
		"name":  "data9",
		"value": 9,
	},
}

var testPayload = []*Packet{
	{
		ID: "ID_0",
		Data: Data{
			"name":  "data0",
			"value": 0,
		},
	},
	{
		ID: "ID_1",
		Data: Data{
			"name":  "data1",
			"value": 1,
		},
	},
	{
		ID: "ID_2",
		Data: Data{
			"name":  "data2",
			"value": 2,
		},
	},
	{
		ID: "ID_3",
		Data: Data{
			"name":  "data3",
			"value": 3,
		},
	},
	{
		ID: "ID_4",
		Data: Data{
			"name":  "data4",
			"value": 4,
		},
	},
	{
		ID: "ID_5",
		Data: Data{
			"name":  "data5",
			"value": 5,
		},
	},
	{
		ID: "ID_6",
		Data: Data{
			"name":  "data6",
			"value": 6,
		},
	},
	{
		ID: "ID_7",
		Data: Data{
			"name":  "data7",
			"value": 7,
		},
	},
	{
		ID: "ID_8",
		Data: Data{
			"name":  "data8",
			"value": 8,
		},
	},
	{
		ID: "ID_9",
		Data: Data{
			"name":  "data9",
			"value": 9,
		},
	},
}

var bufferSize = 0

func Benchmark_Test_New(b *testing.B) {
	out := make(chan []Data)
	channel := make(chan []Data)
	m := New("machine_id", func(c context.Context) chan []Data {
		return channel
	},
		&Option{FIFO: boolP(false)},
		&Option{Idempotent: boolP(true)},
		&Option{Metrics: boolP(true)},
		&Option{Span: boolP(false)},
		&Option{BufferSize: intP(0)},
	).Then(
		NewVertex("node_id1", func(m Data) error {
			if _, ok := m["name"]; !ok {
				b.Errorf("packet missing name %v", m)
				return fmt.Errorf("incorrect data have %v want %v", m, "name field")
			}
			return nil
		}).Then(
			NewVertex("node_id2", func(m Data) error {
				if _, ok := m["name"]; !ok {
					b.Errorf("packet missing name %v", m)
					return fmt.Errorf("incorrect data have %v want %v", m, "name field")
				}
				return nil
			}).Split(
				NewSplitter("route_id", SplitError).
					SplitLeft(
						NewSplitter("route_id", SplitError).
							ThenLeft(
								NewVertex("node_id3", func(m Data) error {
									if _, ok := m["name"]; !ok {
										b.Errorf("packet missing name %v", m)
										return fmt.Errorf("incorrect data have %v want %v", m, "name field")
									}
									return nil
								}).
									Transmit(NewTransmission("terminus_id", func(list []Data) error {
										out <- list
										return nil
									})),
							).
							ThenRight(
								NewVertex("node_id", func(m Data) error {
									b.Errorf("no errors expected")
									return nil
								}).
									Transmit(NewTransmission("terminus_id", func(list []Data) error {
										b.Errorf("no errors expected")
										return nil
									})),
							),
					).
					SplitRight(
						NewSplitter("route_id", SplitError).
							TransmitLeft(NewTransmission("terminus_id", func(list []Data) error {
								b.Errorf("no errors expected")
								return nil
							})).
							TransmitRight(NewTransmission("terminus_id", func(list []Data) error {
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
	count := 50000
	out := make(chan []Data)
	t.Run("Test_New", func(t *testing.T) {

		m := New("machine_id", func(c context.Context) chan []Data {
			channel := make(chan []Data)

			go func() {
				for i := 0; i < count; i++ {
					channel <- testList
				}
			}()

			return channel
		}).Then(
			NewVertex("node_id1", func(m Data) error {
				if _, ok := m["name"]; !ok {
					t.Errorf("packet missing name %v", m)
					return fmt.Errorf("incorrect data have %v want %v", m, "name field")
				}
				return nil
			}).Then(
				NewVertex("node_id2", func(m Data) error {
					if _, ok := m["name"]; !ok {
						t.Errorf("packet missing name %v", m)
						return fmt.Errorf("incorrect data have %v want %v", m, "name field")
					}
					return nil
				}).Split(
					NewSplitter("route_id", SplitError).
						SplitLeft(
							NewSplitter("route_id", SplitError).
								ThenLeft(
									NewVertex("node_id3", func(m Data) error {
										if _, ok := m["name"]; !ok {
											t.Errorf("packet missing name %v", m)
											return fmt.Errorf("incorrect data have %v want %v", m, "name field")
										}
										return nil
									}).
										Transmit(NewTransmission("terminus_id", func(list []Data) error {
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
									NewVertex("node_id", func(m Data) error {
										t.Errorf("no errors expected")
										return nil
									}).
										Transmit(NewTransmission("terminus_id", func(list []Data) error {
											t.Errorf("no errors expected")
											return nil
										})),
								),
						).
						SplitRight(
							NewSplitter("route_id", SplitError).
								TransmitLeft(NewTransmission("terminus_id", func(list []Data) error {
									t.Errorf("no errors expected")
									return nil
								})).
								TransmitRight(NewTransmission("terminus_id", func(list []Data) error {
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
		out := make(chan []Data)

		term := NewTransmission("terminus_id", func(list []Data) error {
			t.Errorf("no errors expected")
			return nil
		})

		m := New("machine_id", func(c context.Context) chan []Data {
			channel := make(chan []Data)

			go func() {
				for i := 0; i < count; i++ {
					channel <- testList
				}
			}()

			return channel
		}, &Option{FIFO: boolP(true)}).Then(
			NewVertex("node_id1", func(m Data) error {
				if _, ok := m["name"]; !ok {
					t.Errorf("packet missing name %v", m)
					return fmt.Errorf("incorrect data have %v want %v", m, "name field")
				}
				return nil
			}).Then(
				NewVertex("node_id2", func(m Data) error {
					if _, ok := m["name"]; !ok {
						t.Errorf("packet missing name %v", m)
						return fmt.Errorf("incorrect data have %v want %v", m, "name field")
					}
					return nil
				}).Split(
					NewSplitter("route_id", SplitError).
						SplitLeft(
							NewSplitter("route_id", SplitError).
								ThenLeft(
									NewVertex("node_id3", func(m Data) error {
										if _, ok := m["name"]; !ok {
											t.Errorf("packet missing name %v", m)
											return fmt.Errorf("incorrect data have %v want %v", m, "name field")
										}
										return nil
									}).
										Transmit(NewTransmission("terminus_id", func(list []Data) error {
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
									NewVertex("node_id", func(m Data) error {
										t.Errorf("no errors expected")
										return nil
									}).
										Transmit(term),
								),
						).
						SplitRight(
							NewSplitter("route_id", SplitError).
								TransmitLeft(term).
								TransmitRight(term),
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
		out := make(chan []Data)

		term := NewTransmission("terminus_id", func(list []Data) error {
			t.Errorf("no errors expected")
			return nil
		})

		m := New("machine_id", func(c context.Context) chan []Data {
			channel := make(chan []Data)

			go func() {
				for i := 0; i < count; i++ {
					channel <- testList
				}
			}()

			return channel
		},
			&Option{FIFO: boolP(true)},
			&Option{Idempotent: boolP(true)},
			&Option{Metrics: boolP(false)},
			&Option{Span: boolP(false)},
			&Option{BufferSize: intP(10)},
		).Then(
			NewVertex("node_id1", func(m Data) error {
				if _, ok := m["name"]; !ok {
					t.Errorf("packet missing name %v", m)
					return fmt.Errorf("incorrect data have %v want %v", m, "name field")
				}
				return nil
			}).Then(
				NewVertex("node_id2", func(m Data) error {
					if _, ok := m["name"]; !ok {
						t.Errorf("packet missing name %v", m)
						return fmt.Errorf("incorrect data have %v want %v", m, "name field")
					}
					return nil
				}).Split(
					NewSplitter("route_id", SplitError).
						SplitLeft(
							NewSplitter("route_id", SplitError).
								ThenLeft(
									NewVertex("node_id3", func(m Data) error {
										if _, ok := m["name"]; !ok {
											t.Errorf("packet missing name %v", m)
											return fmt.Errorf("incorrect data have %v want %v", m, "name field")
										}
										return nil
									}).
										Transmit(NewTransmission("terminus_id", func(list []Data) error {
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
									NewVertex("node_id", func(m Data) error {
										t.Errorf("no errors expected")
										return nil
									}).
										Transmit(term),
								),
						).
						SplitRight(
							NewSplitter("route_id", SplitError).
								TransmitLeft(term).
								TransmitRight(term),
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
		out := make(chan []Data)

		m := New("machine_id", func(c context.Context) chan []Data {
			channel := make(chan []Data)

			go func() {
				for i := 0; i < count; i++ {
					channel <- testList
				}
			}()

			return channel
		}).Split(
			NewSplitter("route_id", SplitError).
				SplitLeft(
					NewSplitter("route_id", SplitError).
						ThenLeft(
							NewVertex("node_id3", func(m Data) error {
								if _, ok := m["name"]; !ok {
									t.Errorf("packet missing name %v", m)
									return fmt.Errorf("incorrect data have %v want %v", m, "name field")
								}
								return nil
							}).
								Transmit(NewTransmission("terminus_id", func(list []Data) error {
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
							NewVertex("node_id", func(m Data) error {
								t.Errorf("no errors expected")
								return nil
							}).
								Transmit(NewTransmission("terminus_id", func(list []Data) error {
									t.Errorf("no errors expected")
									return nil
								})),
						),
				).
				SplitRight(
					NewSplitter("route_id", SplitError).
						TransmitLeft(NewTransmission("terminus_id", func(list []Data) error {
							t.Errorf("no errors expected")
							return nil
						})).
						TransmitRight(NewTransmission("terminus_id", func(list []Data) error {
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

		m := New("machine_id", func(c context.Context) chan []Data {
			channel := make(chan []Data)

			go func() {
				for i := 0; i < count; i++ {
					channel <- []Data{}
				}
			}()

			return channel
		}).
			Transmit(NewTransmission("terminus_id", func(list []Data) error {
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
		out := make(chan []Data)

		m := New("machine_id", func(c context.Context) chan []Data {
			channel := make(chan []Data)

			go func() {
				for i := 0; i < count; i++ {
					channel <- testList
				}
			}()

			return channel
		}).
			Transmit(NewTransmission("terminus_id", func(list []Data) error {
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
		out := make(chan []Data)

		router := NewSplitter("route_id", SplitError).
			TransmitLeft(NewTransmission("terminus_id", func(list []Data) error {
				t.Errorf("no errors expected")
				return nil
			})).
			TransmitRight(NewTransmission("terminus_id", func(list []Data) error {
				t.Errorf("no errors expected")
				return nil
			}))

		m := New("machine_id", func(c context.Context) chan []Data {
			channel := make(chan []Data)

			go func() {
				for i := 0; i < count; i++ {
					channel <- testList
				}
			}()

			return channel
		}).Then(
			NewVertex("node_id1", func(m Data) error {
				if _, ok := m["name"]; !ok {
					t.Errorf("packet missing name %v", m)
					return fmt.Errorf("incorrect data have %v want %v", m, "name field")
				}
				return nil
			}).Then(
				NewVertex("node_id2", func(m Data) error {
					if _, ok := m["name"]; !ok {
						t.Errorf("packet missing name %v", m)
						return fmt.Errorf("incorrect data have %v want %v", m, "name field")
					}
					return nil
				}).Split(
					NewSplitter("route_id", SplitError).
						SplitLeft(
							NewSplitter("route_id", SplitError).
								ThenLeft(
									NewVertex("node_id3", func(m Data) error {
										if _, ok := m["name"]; !ok {
											t.Errorf("packet missing name %v", m)
											return fmt.Errorf("incorrect data have %v want %v", m, "name field")
										}
										return nil
									}).
										Transmit(NewTransmission("terminus_id", func(list []Data) error {
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
									NewVertex("node_id", func(m Data) error {
										t.Errorf("no errors expected")
										return nil
									}).
										Split(router),
								),
						).
						SplitRight(router),
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

		<-time.After(time.Second)

		cancel()

		<-time.After(time.Second * 2)
	})
}

func Test_New_Missing_Termination(t *testing.T) {
	t.Run("Test_New_Missing_Termination", func(t *testing.T) {
		router := NewSplitter("route_id", SplitError).
			TransmitRight(NewTransmission("terminus_id", func(list []Data) error {
				t.Errorf("no errors expected")
				return nil
			}))

		m := New("machine_id", func(c context.Context) chan []Data {
			channel := make(chan []Data)
			return channel
		}).Then(
			NewVertex("node_id1", func(m Data) error {
				if _, ok := m["name"]; !ok {
					t.Errorf("packet missing name %v", m)
					return fmt.Errorf("incorrect data have %v want %v", m, "name field")
				}
				return nil
			}).Then(
				NewVertex("node_id2", func(m Data) error {
					if _, ok := m["name"]; !ok {
						t.Errorf("packet missing name %v", m)
						return fmt.Errorf("incorrect data have %v want %v", m, "name field")
					}
					return nil
				}).Split(
					NewSplitter("route_id", SplitError).
						SplitLeft(
							NewSplitter("route_id", SplitError).
								ThenLeft(
									NewVertex("node_id3", func(m Data) error {
										if _, ok := m["name"]; !ok {
											t.Errorf("packet missing name %v", m)
											return fmt.Errorf("incorrect data have %v want %v", m, "name field")
										}
										return nil
									}),
								).
								ThenRight(
									NewVertex("node_id", func(m Data) error {
										t.Errorf("no errors expected")
										return nil
									}).
										Split(router),
								),
						),
				),
			),
		)

		if err := m.Run(context.Background(), func(s1, s2, s3 string, p []*Packet) {}); err == nil {
			t.Errorf("did not find errors")
		}

		m2 := New("machine_id", func(c context.Context) chan []Data {
			channel := make(chan []Data)
			return channel
		})

		if m2.ID() != "machine_id" {
			t.Errorf("incorrect id have %s want %s", m2.ID(), "machine_id")
		}

		if err := m2.Run(context.Background(), func(s1, s2, s3 string, p []*Packet) {}); err == nil {
			t.Errorf("did not find errors")
		}

		m3 := New("machine_id", func(c context.Context) chan []Data {
			channel := make(chan []Data)
			return channel
		}).Then(
			NewVertex("node_id1", func(m Data) error {
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

		m4 := New("machine_id", func(c context.Context) chan []Data {
			channel := make(chan []Data)
			return channel
		})

		if err := m4.Run(context.Background(), func(s1, s2, s3 string, p []*Packet) {}); err == nil {
			t.Errorf("did not find errors")
		}
	})
}

func Test_New_Duplication(t *testing.T) {
	t.Run("Test_New_Duplication", func(t *testing.T) {
		count := 10000
		out := make(chan []Data)

		m := New("machine_id", func(c context.Context) chan []Data {
			channel := make(chan []Data)

			go func() {
				for i := 0; i < count; i++ {
					channel <- testList
				}
			}()

			return channel
		}).Split(
			NewSplitter("route_id", SplitDuplicate).
				TransmitLeft(NewTransmission("terminus_id", func(list []Data) error {
					for i, packet := range list {
						if !reflect.DeepEqual(packet, testList[i]) {
							t.Errorf("incorrect data have %v want %v", packet, testList[i])
						}
					}
					out <- list
					return nil
				})).
				TransmitRight(NewTransmission("terminus_id", func(list []Data) error {
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
		out := make(chan []Data)

		m := New("machine_id", func(c context.Context) chan []Data {
			channel := make(chan []Data)

			go func() {
				for i := 0; i < count; i++ {
					channel <- testList
				}
			}()

			return channel
		}).Split(
			NewSplitter("route_id", SplitRule(func(m Data) bool { return true }).Handler).
				TransmitLeft(NewTransmission("terminus_id", func(list []Data) error {
					for i, packet := range list {
						if !reflect.DeepEqual(packet, testList[i]) {
							t.Errorf("incorrect data have %v want %v", packet, testList[i])
						}
					}
					out <- list
					return nil
				})).
				TransmitRight(NewTransmission("terminus_id", func(list []Data) error {
					t.Errorf("no errors expected")
					return nil
				})),
		)

		if err := m.Run(context.Background()); err != nil {
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
		out := make(chan []Data)

		node := NewVertex("node_id1", func(m Data) error {
			if _, ok := m["name"]; !ok {
				t.Errorf("packet missing name %v", m)
				return fmt.Errorf("incorrect data have %v want %v", m, "name field")
			}
			return fmt.Errorf("fail everything")
		}).
			Transmit(NewTransmission("terminus_id", func(list []Data) error {
				for i, packet := range list {
					if !reflect.DeepEqual(packet, testList[i]) {
						t.Errorf("incorrect data have %v want %v", packet, testList[i])
					}
				}
				out <- list
				return nil
			}))

		m := New("machine_id", func(c context.Context) chan []Data {
			channel := make(chan []Data)

			go func() {
				for i := 0; i < count; i++ {
					channel <- testList
				}
			}()

			return channel
		}).
			Then(node)

		if err := m.Run(context.Background()); err != nil {
			t.Errorf("did not find errors")
		}

		m2 := New("machine_id", func(c context.Context) chan []Data {
			channel := make(chan []Data)

			go func() {
				for i := 0; i < count; i++ {
					channel <- testList
				}
			}()

			return channel
		}).
			Then(node)

		if err := m2.Run(context.Background()); err != nil {
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
		out := make(chan []Data)

		m := New("machine_id", func(c context.Context) chan []Data {
			channel := make(chan []Data)

			go func() {
				for i := 0; i < count; i++ {
					channel <- testList
				}
			}()

			return channel
		}).Then(
			NewVertex("node_id1", func(m Data) error {
				if _, ok := m["name"]; !ok {
					t.Errorf("packet missing name %v", m)
					return fmt.Errorf("incorrect data have %v want %v", m, "name field")
				}
				return fmt.Errorf("fail everything")
			}).Split(
				NewSplitter("route_id", SplitError).
					TransmitLeft(NewTransmission("terminus_id", func(list []Data) error {
						t.Errorf("no errors expected")
						return nil
					})).
					TransmitRight(NewTransmission("terminus_id", func(list []Data) error {
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

		if err := m.Run(context.Background()); err != nil {
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
		out := make(chan []Data)

		m := New("machine_id", func(c context.Context) chan []Data {
			channel := make(chan []Data)

			go func() {
				for i := 0; i < count; i++ {
					channel <- testList
				}
			}()

			return channel
		}).Split(
			NewSplitter("route_id", SplitRule(func(m Data) bool { return false }).Handler).
				TransmitLeft(NewTransmission("terminus_id", func(list []Data) error {
					t.Errorf("no errors expected")
					return nil
				})).
				TransmitRight(NewTransmission("terminus_id", func(list []Data) error {
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
		out := make(chan []Data)

		m := New("machine_id", func(c context.Context) chan []Data {
			channel := make(chan []Data)

			go func() {
				for i := 0; i < count; i++ {
					channel <- testList
				}
			}()

			return channel
		}).Split(
			NewSplitter("route_id", SplitRule(func(m Data) bool { return false }).Handler).
				ThenLeft(
					NewVertex("node_id1", func(m Data) error {
						if _, ok := m["name"]; !ok {
							t.Errorf("packet missing name %v", m)
							return fmt.Errorf("incorrect data have %v want %v", m, "name field")
						}
						return fmt.Errorf("fail everything")
					}),
				).
				TransmitRight(NewTransmission("terminus_id", func(list []Data) error {
					for i, packet := range list {
						if !reflect.DeepEqual(packet, testList[i]) {
							t.Errorf("incorrect data have %v want %v", packet, testList[i])
						}
					}
					out <- list
					return nil
				})),
		)

		if err := m.Run(context.Background()); err == nil {
			t.Errorf("did not find errors")
		}
	})
}

func Test_New_Rule_Right_Error(t *testing.T) {
	t.Run("Test_New_Rule_Right_Error", func(t *testing.T) {
		count := 10000

		m := New("machine_id", func(c context.Context) chan []Data {
			channel := make(chan []Data)

			go func() {
				for i := 0; i < count; i++ {
					channel <- testList
				}
			}()

			return channel
		}).Split(
			NewSplitter("route_id", SplitRule(func(m Data) bool { return false }).Handler).
				TransmitLeft(NewTransmission("terminus_id", func(list []Data) error {
					t.Errorf("no errors expected")
					return nil
				})).
				ThenRight(
					NewVertex("node_id1", func(m Data) error {
						if _, ok := m["name"]; !ok {
							t.Errorf("packet missing name %v", m)
							return fmt.Errorf("incorrect data have %v want %v", m, "name field")
						}
						return fmt.Errorf("fail everything")
					}),
				),
		)

		if err := m.Run(context.Background()); err == nil {
			t.Errorf("did not find errors")
		}
	})
}

func Test_mergeRecorders(t *testing.T) {
	count := 10000
	out1 := make(chan []*Packet)
	out2 := make(chan []*Packet)

	r1 := func(s1, s2, s3 string, p []*Packet) {
		go func() { out1 <- p }()
	}
	r2 := func(s1, s2, s3 string, p []*Packet) {
		go func() { out2 <- p }()
	}

	r := mergeRecorders(r1, r2)

	for i := 0; i < count; i++ {
		r("test", "test", "test", testPayload)
		<-out1
		<-out2
	}
}
