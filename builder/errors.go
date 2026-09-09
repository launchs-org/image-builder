package main

import "fmt"

// 特定の条件下で必須となる引数が未指定のときのエラーを組み立てる
func errRequired(when, flagName string) error {
	return fmt.Errorf("%s のとき --%s は必須です", when, flagName)
}

// 引数に許可されていない値が指定されたときのエラーを組み立てる
func errInvalidChoice(flagName, got string, allowed ...string) error {
	return fmt.Errorf("--%s には %v のいずれかを指定してください (指定値: %q)", flagName, allowed, got)
}
