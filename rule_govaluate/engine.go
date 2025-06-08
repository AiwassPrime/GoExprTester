package rule_govaluate

import (
	"fmt"
	"math/rand"
	"time"

	"sync"

	"github.com/Knetic/govaluate"
)

/* ---------- 因子模板 ---------- */

type Kind int

const (
	Bool Kind = iota
	String
	Int
)

type FactorTemplate struct {
	Name         string
	Kind         Kind
	SampleValues []interface{}
}

var factorPool = []FactorTemplate{
	// Bool
	{"is_vip", Bool, nil},
	{"blacklisted", Bool, nil},
	{"email_verified", Bool, nil},
	{"high_risk_ip", Bool, nil},
	// String
	{"env", String, []interface{}{"prod", "staging", "test_env"}},
	{"payment_method", String, []interface{}{"ABCD", "XYZ", "PAYPAL", "STRIPE"}},
	// Int
	{"user_id", Int, []interface{}{12345, 67890, 13579, 24680}},
}

/* ---------- RuleEngine 与 Rule (Govaluate) ---------- */

type Rule struct {
	ID         string
	ExprString string
	Expr       *govaluate.EvaluableExpression
}

type RuleEngine struct {
	rules sync.Map // id -> *Rule
}

// AddRule 解析并加入/替换一条规则
func (re *RuleEngine) AddRule(id, exprStr string) error {
	parsedExpr, err := govaluate.NewEvaluableExpression(exprStr)
	if err != nil {
		return err
	}
	re.rules.Store(id, &Rule{
		ID:         id,
		ExprString: exprStr,
		Expr:       parsedExpr,
	})
	return nil
}

// Match 遍历执行全部规则并返回命中 ID
func (re *RuleEngine) Match(input map[string]interface{}) []string {
	var hits []string
	re.rules.Range(func(_, value any) bool {
		r := value.(*Rule)
		out, err := r.Expr.Evaluate(input)
		if err == nil {
			if ok, _ := out.(bool); ok {
				hits = append(hits, r.ID)
			}
		}
		return true
	})
	return hits
}

/* ---------- 随机规则注入 ---------- */

func InjectRandomRules(re *RuleEngine, count int) error {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	for i := 0; i < count; i++ {
		ruleID := fmt.Sprintf("auto-%d", i+1)
		exprStr := randomExpr(r, 5) // ≤ 5 因子
		if err := re.AddRule(ruleID, exprStr); err != nil {
			return fmt.Errorf("编译规则 %s 失败: %w", ruleID, err)
		} else {
			fmt.Printf("编译规则 %s 成功: %s\n", ruleID, exprStr)
		}
	}
	return nil
}

// ---- 表达式生成（与前版一致，只是保留了 "not/and/or" 语义） ----

func randomExpr(r *rand.Rand, maxFactors int) string {
	n := r.Intn(maxFactors) + 1
	perm := r.Perm(len(factorPool))[:n]
	var factors []FactorTemplate
	for _, idx := range perm {
		factors = append(factors, factorPool[idx])
	}
	return buildSubExpr(r, factors)
}

func buildSubExpr(r *rand.Rand, factors []FactorTemplate) string {
	if len(factors) == 1 {
		frag := snippet(r, factors[0])
		if r.Float64() < 0.3 { // 30% 概率加 not
			return "! (" + frag + ")"
		}
		return frag
	}
	split := r.Intn(len(factors)-1) + 1
	left := buildSubExpr(r, factors[:split])
	right := buildSubExpr(r, factors[split:])
	op := "&&"
	if r.Float64() < 0.5 {
		op = "||"
	}
	return fmt.Sprintf("(%s %s %s)", left, op, right)
}

func snippet(r *rand.Rand, f FactorTemplate) string {
	switch f.Kind {
	case Bool:
		// Govaluate 不支持裸变量，必须写成 == true 或 == false
		return fmt.Sprintf("%s == true", f.Name)
	case String:
		v := f.SampleValues[r.Intn(len(f.SampleValues))].(string)
		return fmt.Sprintf("%s == \"%s\"", f.Name, v)
	case Int:
		v := f.SampleValues[r.Intn(len(f.SampleValues))].(int)
		return fmt.Sprintf("%s == %d", f.Name, v)
	default:
		return f.Name
	}
}

/* ---------- 随机数据生成 & Benchmark ---------- */

func GenRandomInputs(n int) []map[string]interface{} {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	rows := make([]map[string]interface{}, n)
	for i := 0; i < n; i++ {
		row := make(map[string]interface{}, len(factorPool))
		for _, f := range factorPool {
			switch f.Kind {
			case Bool:
				row[f.Name] = r.Intn(2) == 0
			case String:
				row[f.Name] = f.SampleValues[r.Intn(len(f.SampleValues))]
			case Int:
				if r.Float64() < 0.8 {
					row[f.Name] = f.SampleValues[r.Intn(len(f.SampleValues))]
				} else {
					row[f.Name] = r.Intn(90000) + 10000
				}
			}
		}
		rows[i] = row
	}
	return rows
}

func BenchmarkMatch(re *RuleEngine, inputs []map[string]interface{}) time.Duration {
	start := time.Now()
	for _, in := range inputs {
		_ = re.Match(in)
	}
	return time.Since(start) / time.Duration(len(inputs))
}
