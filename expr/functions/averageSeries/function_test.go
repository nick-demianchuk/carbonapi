package averageSeries

import (
	"go.uber.org/zap"
	"testing"
	"time"

	"math"

	"github.com/bookingcom/carbonapi/expr/helper"
	"github.com/bookingcom/carbonapi/expr/metadata"
	"github.com/bookingcom/carbonapi/expr/types"
	"github.com/bookingcom/carbonapi/pkg/parser"
	th "github.com/bookingcom/carbonapi/tests"
)

func init() {
	md := New("")
	evaluator := th.EvaluatorFromFunc(md[0].F)
	metadata.SetEvaluator(evaluator)
	helper.SetEvaluator(evaluator)
	for _, m := range md {
		metadata.RegisterFunction(m.Name, m.F, zap.NewNop())
	}
}

func TestAverageSeries(t *testing.T) {
	now32 := int32(time.Now().Unix())

	tests := []th.EvalTestItem{
		{
			"averageSeries(metric1,metric2,metric3)",
			map[parser.MetricRequest][]*types.MetricData{
				{"metric1", 0, 1}: {types.MakeMetricData("metric1", []float64{1, math.NaN(), 2, 3, 4, 5}, 1, now32)},
				{"metric2", 0, 1}: {types.MakeMetricData("metric2", []float64{2, math.NaN(), 3, math.NaN(), 5, 6}, 1, now32)},
				{"metric3", 0, 1}: {types.MakeMetricData("metric3", []float64{3, math.NaN(), 4, 5, 6, math.NaN()}, 1, now32)},
			},
			[]*types.MetricData{types.MakeMetricData("averageSeries(metric1,metric2,metric3)",
				[]float64{2, math.NaN(), 3, 4, 5, 5.5}, 1, now32)},
		},
	}

	for _, tt := range tests {
		tt := tt
		testName := tt.Target
		t.Run(testName, func(t *testing.T) {
			th.TestEvalExpr(t, &tt)
		})
	}

}
