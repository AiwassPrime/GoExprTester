package main

import (
	"fmt"
	"goexprtester/rule_expr"
)

func main() {
	engine := rule_expr.NewRuleEngine()

	// 1. 注入 10k 条随机规则
	if err := rule_expr.InjectRandomRules(engine, 10000); err != nil {
		panic(err)
	}

	// 2. 生成 20k 条随机输入
	inputs := rule_expr.GenRandomInputs(100)

	// 3. Benchmark
	avg := rule_expr.BenchmarkMatch(engine, inputs)
	fmt.Printf("平均每条数据匹配耗时: %s (%d ns)\n", avg, avg.Nanoseconds())
}
