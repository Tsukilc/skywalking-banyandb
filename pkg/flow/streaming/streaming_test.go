// Licensed to Apache Software Foundation (ASF) under one or more contributor
// license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright
// ownership. Apache Software Foundation (ASF) licenses this file to you under
// the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package streaming

import (
	"context"
	"sync"
	"time"

	g "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"

	"github.com/apache/skywalking-banyandb/pkg/convert"
	"github.com/apache/skywalking-banyandb/pkg/flow"
	"github.com/apache/skywalking-banyandb/pkg/test/flags"
	flowTest "github.com/apache/skywalking-banyandb/pkg/test/flow"
)

// numberRange generates a slice with `count` number of integers starting from `begin`,
// i.e. [begin, begin + count).
func numberRange(begin, count int) []int {
	result := make([]int, 0)
	for i := 0; i < count; i++ {
		result = append(result, begin+i)
	}
	return result
}

var _ = g.Describe("Streaming", func() {
	var (
		f     flow.Flow
		snk   *slice
		errCh <-chan error
	)

	g.AfterEach(func() {
		gomega.Expect(f.Close()).Should(gomega.Succeed())
		gomega.Consistently(errCh).ShouldNot(gomega.Receive())
	})

	g.Context("With Filter operator", func() {
		var (
			filter flow.UnaryFunc[bool]

			input = flowTest.NewSlice(numberRange(0, 10))
		)

		g.JustBeforeEach(func() {
			snk = newSlice()
			f = New("test", input).
				Filter(filter).
				To(snk)
			errCh = f.Open()
			gomega.Expect(errCh).ShouldNot(gomega.BeNil())
		})

		g.When("Given a odd filter", func() {
			g.BeforeEach(func() {
				filter = func(_ context.Context, i interface{}) bool {
					return i.(int)%2 == 0
				}
			})

			g.It("Should filter odd number", func() {
				gomega.Eventually(func(g gomega.Gomega) {
					g.Expect(snk.Value()).Should(gomega.Equal([]interface{}{
						flow.NewStreamRecordWithoutTS(0),
						flow.NewStreamRecordWithoutTS(2),
						flow.NewStreamRecordWithoutTS(4),
						flow.NewStreamRecordWithoutTS(6),
						flow.NewStreamRecordWithoutTS(8),
					}))
				}, flags.EventuallyTimeout).Should(gomega.Succeed())
			})
		})
	})

	g.Context("With Mapper operator", func() {
		var (
			mapper flow.UnaryFunc[any]

			input = flowTest.NewSlice(numberRange(0, 10))
		)

		g.JustBeforeEach(func() {
			snk = newSlice()
			f = New("test", input).
				Map(mapper).
				To(snk)
			errCh = f.Open()
			gomega.Expect(errCh).ShouldNot(gomega.BeNil())
		})

		g.When("given a multiplier", func() {
			g.BeforeEach(func() {
				mapper = func(_ context.Context, i interface{}) interface{} {
					return i.(int) * 2
				}
			})

			g.It("Should multiply by 2", func() {
				gomega.Eventually(func(g gomega.Gomega) {
					g.Expect(snk.Value()).Should(gomega.Equal([]interface{}{
						flow.NewStreamRecordWithoutTS(0),
						flow.NewStreamRecordWithoutTS(2),
						flow.NewStreamRecordWithoutTS(4),
						flow.NewStreamRecordWithoutTS(6),
						flow.NewStreamRecordWithoutTS(8),
						flow.NewStreamRecordWithoutTS(10),
						flow.NewStreamRecordWithoutTS(12),
						flow.NewStreamRecordWithoutTS(14),
						flow.NewStreamRecordWithoutTS(16),
						flow.NewStreamRecordWithoutTS(18),
					}))
				}, flags.EventuallyTimeout).Should(gomega.Succeed())
			})
		})
	})

	g.Context("With TopN operator order by ASC", func() {
		type record struct {
			service  string
			instance string
			value    int
		}

		var input []flow.StreamRecord

		g.JustBeforeEach(func() {
			snk = newSlice()

			f = New("test", flowTest.NewSlice(input)).
				Map(flow.UnaryFunc[any](func(_ context.Context, item interface{}) interface{} {
					// groupBy
					return flow.Data{item.(*record).service, int64(item.(*record).value), item.(*record).service + item.(*record).instance}
				})).
				Window(NewTumblingTimeWindows(15*time.Second, 15*time.Second)).
				TopN(3, WithKeyExtractor(func(record flow.StreamRecord) uint64 {
					return convert.HashStr(record.Data().(flow.Data)[2].(string))
				}),
					WithSortKeyExtractor(func(record flow.StreamRecord) int64 {
						return record.Data().(flow.Data)[1].(int64)
					}), OrderBy(ASC), WithGroupKeyExtractor(func(record flow.StreamRecord) string {
						return record.Data().(flow.Data)[0].(string)
					})).
				To(snk)

			errCh = f.Open()
			gomega.Expect(errCh).ShouldNot(gomega.BeNil())
		})

		g.When("Bottom3", func() {
			g.BeforeEach(func() {
				input = []flow.StreamRecord{
					flow.NewStreamRecord(&record{"e2e-service-provider", "instance-001", 10000}, 1000),
					flow.NewStreamRecord(&record{"e2e-service-consumer", "instance-001", 9900}, 2000),
					flow.NewStreamRecord(&record{"e2e-service-provider", "instance-002", 9800}, 3000),
					flow.NewStreamRecord(&record{"e2e-service-consumer", "instance-002", 9700}, 4000),
					flow.NewStreamRecord(&record{"e2e-service-provider", "instance-003", 9700}, 5000),
					flow.NewStreamRecord(&record{"e2e-service-consumer", "instance-004", 9600}, 6000),
					flow.NewStreamRecord(&record{"e2e-service-consumer", "instance-001", 9500}, 7000),
					flow.NewStreamRecord(&record{"e2e-service-provider", "instance-002", 9800}, 61000),
				}
			})

			g.It("Should take bottom 3 elements", func() {
				gomega.Eventually(func(g gomega.Gomega) {
					g.Expect(len(snk.Value())).Should(gomega.BeNumerically(">=", 1))
					// e2e-service-consumer Group
					g.Expect(snk.Value()[0].(flow.StreamRecord).Data().(map[string][]*Tuple2)["e2e-service-consumer"]).Should(gomega.BeEquivalentTo([]*Tuple2{
						{int64(9500), flow.NewStreamRecord(flow.Data{"e2e-service-consumer", int64(9500), "e2e-service-consumerinstance-001"}, 7000)},
						{int64(9600), flow.NewStreamRecord(flow.Data{"e2e-service-consumer", int64(9600), "e2e-service-consumerinstance-004"}, 6000)},
						{int64(9700), flow.NewStreamRecord(flow.Data{"e2e-service-consumer", int64(9700), "e2e-service-consumerinstance-002"}, 4000)},
					}))
					// e2e-service-provider Group
					g.Expect(snk.Value()[0].(flow.StreamRecord).Data().(map[string][]*Tuple2)["e2e-service-provider"]).Should(gomega.BeEquivalentTo([]*Tuple2{
						{int64(9700), flow.NewStreamRecord(flow.Data{"e2e-service-provider", int64(9700), "e2e-service-providerinstance-003"}, 5000)},
						{int64(9800), flow.NewStreamRecord(flow.Data{"e2e-service-provider", int64(9800), "e2e-service-providerinstance-002"}, 3000)},
						{int64(10000), flow.NewStreamRecord(flow.Data{"e2e-service-provider", int64(10000), "e2e-service-providerinstance-001"}, 1000)},
					}))
				}).WithTimeout(flags.EventuallyTimeout).Should(gomega.Succeed())
			})
		})
	})

	g.Context("With TopN operator order by DESC", func() {
		type record struct {
			service  string
			instance string
			value    int
		}

		var input []flow.StreamRecord

		g.JustBeforeEach(func() {
			snk = newSlice()

			f = New("test", flowTest.NewSlice(input)).
				Map(flow.UnaryFunc[any](func(_ context.Context, item interface{}) interface{} {
					// groupBy
					return flow.Data{item.(*record).service, int64(item.(*record).value), item.(*record).service + item.(*record).instance}
				})).
				Window(NewTumblingTimeWindows(15*time.Second, 15*time.Second)).
				TopN(3, WithKeyExtractor(func(record flow.StreamRecord) uint64 {
					return convert.HashStr(record.Data().(flow.Data)[2].(string))
				}), WithSortKeyExtractor(func(record flow.StreamRecord) int64 {
					return record.Data().(flow.Data)[1].(int64)
				}), WithGroupKeyExtractor(func(record flow.StreamRecord) string {
					return record.Data().(flow.Data)[0].(string)
				})).
				To(snk)

			errCh = f.Open()
			gomega.Expect(errCh).ShouldNot(gomega.BeNil())
		})

		g.When("Top3", func() {
			g.BeforeEach(func() {
				input = []flow.StreamRecord{
					flow.NewStreamRecord(&record{"e2e-service-provider", "instance-001", 10000}, 1000),
					flow.NewStreamRecord(&record{"e2e-service-consumer", "instance-001", 9900}, 2000),
					flow.NewStreamRecord(&record{"e2e-service-provider", "instance-002", 9800}, 3000),
					flow.NewStreamRecord(&record{"e2e-service-consumer", "instance-002", 9700}, 4000),
					flow.NewStreamRecord(&record{"e2e-service-provider", "instance-003", 9700}, 5000),
					flow.NewStreamRecord(&record{"e2e-service-consumer", "instance-004", 9600}, 6000),
					flow.NewStreamRecord(&record{"e2e-service-consumer", "instance-001", 9500}, 7000),
					flow.NewStreamRecord(&record{"e2e-service-provider", "instance-002", 9800}, 61000),
				}
			})

			g.It("Should take top 3 elements", func() {
				gomega.Eventually(func(g gomega.Gomega) {
					g.Expect(len(snk.Value())).Should(gomega.BeNumerically(">=", 1))
					// e2e-service-consumer Group
					g.Expect(snk.Value()[0].(flow.StreamRecord).Data().(map[string][]*Tuple2)["e2e-service-consumer"]).Should(gomega.BeEquivalentTo([]*Tuple2{
						{int64(9700), flow.NewStreamRecord(flow.Data{"e2e-service-consumer", int64(9700), "e2e-service-consumerinstance-002"}, 4000)},
						{int64(9600), flow.NewStreamRecord(flow.Data{"e2e-service-consumer", int64(9600), "e2e-service-consumerinstance-004"}, 6000)},
						{int64(9500), flow.NewStreamRecord(flow.Data{"e2e-service-consumer", int64(9500), "e2e-service-consumerinstance-001"}, 7000)},
					}))
					// e2e-service-provider Group
					g.Expect(snk.Value()[0].(flow.StreamRecord).Data().(map[string][]*Tuple2)["e2e-service-provider"]).Should(gomega.BeEquivalentTo([]*Tuple2{
						{int64(10000), flow.NewStreamRecord(flow.Data{"e2e-service-provider", int64(10000), "e2e-service-providerinstance-001"}, 1000)},
						{int64(9800), flow.NewStreamRecord(flow.Data{"e2e-service-provider", int64(9800), "e2e-service-providerinstance-002"}, 3000)},
						{int64(9700), flow.NewStreamRecord(flow.Data{"e2e-service-provider", int64(9700), "e2e-service-providerinstance-003"}, 5000)},
					}))
				}).WithTimeout(flags.EventuallyTimeout).Should(gomega.Succeed())
			})
		})
	})
})

var _ flow.Sink = (*slice)(nil)

type slice struct {
	in    chan flow.StreamRecord
	slice []interface{}
	flow.ComponentState
	sync.RWMutex
}

func newSlice() *slice {
	return &slice{
		slice: make([]interface{}, 0),
		in:    make(chan flow.StreamRecord),
	}
}

func (s *slice) Value() []interface{} {
	s.RLock()
	defer s.RUnlock()
	return s.slice
}

func (s *slice) In() chan<- flow.StreamRecord {
	return s.in
}

func (s *slice) Setup(ctx context.Context) error {
	go s.run(ctx)

	return nil
}

func (s *slice) run(ctx context.Context) {
	s.Add(1)
	defer func() {
		s.Done()
	}()
	for {
		select {
		case item, ok := <-s.in:
			if !ok {
				return
			}
			s.Lock()
			s.slice = append(s.slice, item)
			s.Unlock()
		case <-ctx.Done():
			return
		}
	}
}

func (s *slice) Teardown(_ context.Context) error {
	s.Wait()
	return nil
}
