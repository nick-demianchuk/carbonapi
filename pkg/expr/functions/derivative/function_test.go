package derivative

import (
	"testing"
	"time"

	"go.uber.org/zap"

	"math"

	"github.com/bookingcom/carbonapi/pkg/expr/helper"
	"github.com/bookingcom/carbonapi/pkg/expr/metadata"
	"github.com/bookingcom/carbonapi/pkg/expr/types"
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

func TestDerivative(t *testing.T) {
	now32 := int32(time.Now().Unix())

	tests := []th.EvalTestItem{
		{
			"derivative(metric1)",
			map[parser.MetricRequest][]*types.MetricData{
				{"metric1", 0, 1}: {types.MakeMetricData("metric1", []float64{2, 4, 6, 1, 4, math.NaN(), 8}, 1, now32)},
			},
			[]*types.MetricData{types.MakeMetricData("derivative(metric1)",
				[]float64{math.NaN(), 2, 2, -5, 3, math.NaN(), 4}, 1, now32)},
		},
		{
			"derivative(metric1)",
			map[parser.MetricRequest][]*types.MetricData{
				{"metric1", 0, 1}: {types.MakeMetricData("metric1", []float64{math.NaN(), 4, 6, 1, 4, math.NaN(), 8}, 1, now32)},
			},
			[]*types.MetricData{types.MakeMetricData("derivative(metric1)",
				[]float64{math.NaN(), math.NaN(), 2, -5, 3, math.NaN(), 4}, 1, now32)},
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
