# 数据类型和控制流

Go语言提供多种数据类型，包括基本数据类型（如整数、浮点数、布尔值和字符串）以及复合数据类型（如数组、切片、映射、结构体和接口）。控制流语句允许我们执行不同的逻辑，比如条件判断和循环。

## 基本数据类型
- **整数**: int、int8、int16、int32、int64
- **浮点数**: float32、float64
- **布尔值**: bool
- **字符串**: string

## 控制流语句
- **if 语句**: 用于条件判断。
- **switch 语句**: 替代多重条件判断。
- **for 循环**: 提供普通和范围形式的循环。
- **defer 语句**: 用于延迟执行函数，通常用于清理资源。

## 示例代码
```go
package main
import "fmt"

func main() {
    // 使用 if 语句
    x := 10
    if x > 5 {
        fmt.Println("x 是大于 5 的数")
    }

    // 使用 switch 语句
    switch x {
    case 10:
        fmt.Println("x 是 10")
    default:
        fmt.Println("x 不是 10")
    }
}
```