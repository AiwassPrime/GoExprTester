package rule_expr

import (
	"fmt"
	"math/rand"
	"time"

	"sync"

	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"
)

/* ---------- 因子模板 ---------- */

type Kind int

const (
	Bool Kind = iota
	String
	Int
)

// FactorTemplate 描述一类可用于规则的因子
type FactorTemplate struct {
	Name         string        // 变量名
	Kind         Kind          // Bool / String / Int
	SampleValues []interface{} // 枚举值，用于生成 "==" 常量
}

// 现实场景因子池
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

/* ---------- RuleEngine 与 Rule ---------- */

type Rule struct {
	ID      string
	ExprStr string
	Program *vm.Program
}

type RuleEngine struct {
	rules         sync.Map // id -> *Rule
	rulesNoneSync map[string]*Rule
}

func NewRuleEngine() *RuleEngine {
	return &RuleEngine{
		rules:         sync.Map{},
		rulesNoneSync: make(map[string]*Rule),
	}
}

// AddRule 编译并加入（或覆盖）一条规则
func (re *RuleEngine) AddRule(id, exprStr string) error {
	p, err := expr.Compile(exprStr, expr.AsBool())
	if err != nil {
		return err
	}
	re.rules.Store(id, &Rule{
		ID:      id,
		ExprStr: exprStr,
		Program: p,
	})
	re.rulesNoneSync[id] = &Rule{
		ID:      id,
		ExprStr: exprStr,
		Program: p,
	}
	return nil
}

// Match 遍历执行全部规则，返回命中 ID
func (re *RuleEngine) Match(input map[string]interface{}) []string {
	var hits []string
	re.rules.Range(func(_, value any) bool {
		r := value.(*Rule)
		out, _ := expr.Run(r.Program, input)
		if out.(bool) {
			hits = append(hits, r.ID)
		}
		return true
	})
	return hits
}

func (re *RuleEngine) MatchNoneSync(input map[string]interface{}) []string {
	var hits []string
	for _, r := range re.rulesNoneSync {
		out, _ := expr.Run(r.Program, input)
		if out.(bool) {
			hits = append(hits, r.ID)
		}
	}
	return hits
}

/* ---------- 随机规则注入 ---------- */

// InjectRandomRules 生成 count 条随机规则
func InjectRandomRules(re *RuleEngine, count int) error {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	for i := 0; i < count; i++ {
		ruleID := fmt.Sprintf("auto-%d", i+1)
		exprStr := randomExpr(r, 5) // ≤5 因子
		if err := re.AddRule(ruleID, exprStr); err != nil {
			return fmt.Errorf("编译规则 %s 失败: %w", ruleID, err)
		} else {
			fmt.Printf("编译规则 %s 成功: %s\n", ruleID, exprStr)
		}
	}
	return nil
}

// randomExpr 随机拼装布尔表达式
func randomExpr(r *rand.Rand, maxFactors int) string {
	// 1. 随机选取 1~maxFactors 个不同因子
	n := r.Intn(maxFactors) + 1
	perm := r.Perm(len(factorPool))[:n]
	var factors []FactorTemplate
	for _, idx := range perm {
		factors = append(factors, factorPool[idx])
	}
	// 2. 递归拼装
	return buildSubExpr(r, factors)
}

// buildSubExpr 递归生成子表达式
func buildSubExpr(r *rand.Rand, factors []FactorTemplate) string {
	if len(factors) == 1 {
		frag := snippet(r, factors[0])
		// 30% 概率前置 not
		if r.Float64() < 0.3 {
			return "not (" + frag + ")"
		}
		return frag
	}
	split := r.Intn(len(factors)-1) + 1
	left := buildSubExpr(r, factors[:split])
	right := buildSubExpr(r, factors[split:])
	op := "and"
	if r.Float64() < 0.5 {
		op = "or"
	}
	return fmt.Sprintf("(%s %s %s)", left, op, right)
}

// snippet 产生单个因子的表达式片段
func snippet(r *rand.Rand, f FactorTemplate) string {
	switch f.Kind {
	case Bool:
		return f.Name
	case String:
		v := f.SampleValues[r.Intn(len(f.SampleValues))].(string)
		return fmt.Sprintf("%s == %q", f.Name, v)
	case Int:
		v := f.SampleValues[r.Intn(len(f.SampleValues))].(int)
		return fmt.Sprintf("%s == %d", f.Name, v)
	default:
		return f.Name
	}
}

/* ---------- 随机数据生成 & Benchmark ---------- */

// GenRandomInputs 生成 n 条随机测试数据
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
				// 80% 概率用样例值，20% 用随机 5 位数
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

// BenchmarkMatch 顺序匹配全部规则
func BenchmarkMatch(re *RuleEngine, inputs []map[string]interface{}) time.Duration {
	start := time.Now()
	for _, in := range inputs {
		_ = re.MatchNoneSync(in)
	}
	return time.Since(start) / time.Duration(len(inputs))
}
