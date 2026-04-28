package main

import (
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
)

// LenType 表示字段长度类型
type LenType int

const (
	// FIXED 固定长度字段
	FIXED LenType = iota
	// LLVAR 两位长度前缀的可变长字段
	LLVAR
	// LLLVAR 三位长度前缀的可变长字段
	LLLVAR
)

// FieldDef 定义 ISO8583 字段的属性
type FieldDef struct {
	ID     int      // 域编号
	Name   string   // 域名称
	Type   LenType  // 长度类型
	MaxLen int      // 最大长度或固定长度
}

// spec 定义各个域的报文规范
var spec = map[int]FieldDef{
	2:  {2, "主账号(PAN)", LLVAR, 19},
	3:  {3, "交易处理码", FIXED, 6},
	4:  {4, "交易金额", FIXED, 12},
	11: {11, "系统跟踪号", FIXED, 6},
	12: {12, "交易时间(hhmmss)", FIXED, 6},
	13: {13, "交易日期(MMdd)", FIXED, 4},
	14: {14, "卡有效期(yyMM)", FIXED, 4},
	22: {22, "服务点输入方式", FIXED, 3},
	23: {23, "卡序列号", FIXED, 3},
	25: {25, "服务点条件码", FIXED, 2},
	26: {26, "服务点PIN获取码", FIXED, 2},
	32: {32, "受理机构代码", LLVAR, 11},
	37: {37, "检索参考号", FIXED, 12},
	38: {38, "授权码", FIXED, 6},
	39: {39, "应答码", FIXED, 2},
	41: {41, "终端号", FIXED, 8},
	42: {42, "商户号", FIXED, 15},
	43: {43, "商户名称地址", LLVAR, 40},
	49: {49, "货币代码", FIXED, 3},
	52: {52, "PIN密文", FIXED, 16},
	54: {54, "附加金额", LLLVAR, 20},
	55: {55, "ICC数据", LLLVAR, 255},
	60: {60, "自定义域", LLLVAR, 60},
}

// ISO8583Message 表示一条 ISO8583 报文
type ISO8583Message struct {
	MTI    string         // 报文类型标识
	Fields map[int]string // 域数据
}

// NewMessage 创建一条新的 ISO8583 报文
func NewMessage(mti string) *ISO8583Message {
	return &ISO8583Message{MTI: mti, Fields: make(map[int]string)}
}

// Set 设置指定域的值
func (m *ISO8583Message) Set(fieldID int, value string) { m.Fields[fieldID] = value }

// Get 获取指定域的值
func (m *ISO8583Message) Get(fieldID int) string { return m.Fields[fieldID] }

// buildBitmap 根据字段集合生成 64 位 bitmap，并转成十六进制字符串
func buildBitmap(fields map[int]string) string {
	bitmap := make([]byte, 8)
	for fid := range fields {
		if fid >= 1 && fid <= 64 {
			bitmap[(fid-1)/8] |= 1 << (7 - uint((fid-1)%8))
		}
	}
	return strings.ToUpper(hex.EncodeToString(bitmap))
}

// parseBitmap 将 bitmap 的十六进制字符串解析为字段编号列表
func parseBitmap(bitmapHex string) ([]int, error) {
	data, err := hex.DecodeString(bitmapHex)
	if err != nil {
		return nil, fmt.Errorf("bitmap hex decode error: %v", err)
	}
	var fields []int
	for i, b := range data {
		for bit := 7; bit >= 0; bit-- {
			if b&(1<<uint(bit)) != 0 {
				fields = append(fields, i*8+(7-bit)+1)
			}
		}
	}
	return fields, nil
}

// Pack 将 ISO8583Message 按规范组装成字符串报文
func Pack(msg *ISO8583Message) (string, error) {
	var buf strings.Builder

	// 写入 MTI
	buf.WriteString(msg.MTI)

	// 生成 bitmap 并写入
	bitmapHex := buildBitmap(msg.Fields)
	buf.WriteString(bitmapHex)

	// 解析 bitmap，按字段顺序依次写入域内容
	fields, _ := parseBitmap(bitmapHex)
	for _, fid := range fields {
		value, ok := msg.Fields[fid]
		if !ok {
			continue
		}

		def, exists := spec[fid]
		if !exists {
			return "", fmt.Errorf("field %d not defined in spec", fid)
		}

		switch def.Type {
		case FIXED:
			// 固定长度字段：左对齐，右侧补空格，再截断到指定长度
			padded := fmt.Sprintf("%-*s", def.MaxLen, value)
			buf.WriteString(padded[:def.MaxLen])

		case LLVAR:
			// 2 位长度前缀 + 内容
			buf.WriteString(fmt.Sprintf("%02d", len(value)))
			buf.WriteString(value)

		case LLLVAR:
			// 3 位长度前缀 + 内容
			buf.WriteString(fmt.Sprintf("%03d", len(value)))
			buf.WriteString(value)
		}
	}

	return buf.String(), nil
}

// Unpack 将原始报文字符串解析为 ISO8583Message
func Unpack(raw string) (*ISO8583Message, error) {
	msg := &ISO8583Message{Fields: make(map[int]string)}
	pos := 0

	// 解析 MTI，长度固定 4 字节
	if len(raw) < 4 {
		return nil, fmt.Errorf("message too short")
	}
	msg.MTI = raw[pos : pos+4]
	pos += 4

	// 读取 bitmap，固定 16 个十六进制字符（8 字节）
	if len(raw) < pos+16 {
		return nil, fmt.Errorf("message too short for bitmap")
	}
	bitmapHex := raw[pos : pos+16]
	pos += 16

	// 从 bitmap 中解析出字段列表
	fields, err := parseBitmap(bitmapHex)
	if err != nil {
		return nil, err
	}

	// 按字段类型逐个解包
	for _, fid := range fields {
		def, exists := spec[fid]
		if !exists {
			return nil, fmt.Errorf("field %d not defined in spec", fid)
		}

		switch def.Type {
		case FIXED:
			// 固定长度字段直接截取指定长度
			if pos+def.MaxLen > len(raw) {
				return nil, fmt.Errorf("field %d: not enough data", fid)
			}
			msg.Fields[fid] = strings.TrimRight(raw[pos:pos+def.MaxLen], " ")
			pos += def.MaxLen

		case LLVAR:
			// 先读取两位长度，再读取对应内容
			if pos+2 > len(raw) {
				return nil, fmt.Errorf("field %d: not enough data for LL", fid)
			}
			l, _ := strconv.Atoi(raw[pos : pos+2])
			pos += 2
			if pos+l > len(raw) {
				return nil, fmt.Errorf("field %d: not enough data", fid)
			}
			msg.Fields[fid] = raw[pos : pos+l]
			pos += l

		case LLLVAR:
			// 先读取三位长度，再读取对应内容
			if pos+3 > len(raw) {
				return nil, fmt.Errorf("field %d: not enough data for LLL", fid)
			}
			l, _ := strconv.Atoi(raw[pos : pos+3])
			pos += 3
			if pos+l > len(raw) {
				return nil, fmt.Errorf("field %d: not enough data", fid)
			}
			msg.Fields[fid] = raw[pos : pos+l]
			pos += l
		}
	}

	return msg, nil
}

// PrintMessage 以表格形式打印报文内容
func PrintMessage(msg *ISO8583Message) {
	fmt.Println("┌──────┬──────────────────┬────────────────────────┐")
	fmt.Printf("│ MTI  │ %-16s │ %-22s │\n", "消息类型", msg.MTI)
	fmt.Println("├──────┼──────────────────┼────────────────────────┤")

	for fid := 1; fid <= 128; fid++ {
		val, ok := msg.Fields[fid]
		if !ok {
			continue
		}

		def, exists := spec[fid]
		name := "未知"
		if exists {
			name = def.Name
		}

		fmt.Printf("│ %4d │ %-16s │ %-22s │\n", fid, name, val)
	}

	fmt.Println("└──────┴──────────────────┴────────────────────────┘")
}

func main() {
	fmt.Println("====== POS消费交易请求 (0200) ======\n")

	// 构造一条请求报文
	req := NewMessage("0200")
	req.Set(2, "6222021234567890123")
	req.Set(3, "000000")
	req.Set(4, "000000010000")
	req.Set(11, "000001")
	req.Set(12, "143025")
	req.Set(13, "0428")
	req.Set(14, "2612")
	req.Set(22, "051")
	req.Set(25, "00")
	req.Set(26, "12")
	req.Set(41, "POS00001")
	req.Set(42, "898440153110001")
	req.Set(49, "156")
	req.Set(52, "AB12CD34EF56GH78")
	req.Set(60, "22000001")

	// 组包
	packed, err := Pack(req)
	if err != nil {
		fmt.Println("组包错误:", err)
		return
	}

	fmt.Printf("报文(%d bytes): %s\n\n", len(packed), packed)
	PrintMessage(req)

	fmt.Println("\n====== 解析报文 ======\n")

	// 解包
	parsed, err := Unpack(packed)
	if err != nil {
		fmt.Println("解包错误:", err)
		return
	}
	PrintMessage(parsed)

	fmt.Println("\n====== 交易应答 (0210) ======\n")

	// 构造一条应答报文
	resp := NewMessage("0210")
	resp.Set(2, "6222021234567890123")
	resp.Set(3, "000000")
	resp.Set(4, "000000010000")
	resp.Set(11, "000001")
	resp.Set(37, "042814302500")
	resp.Set(38, "A12345")
	resp.Set(39, "00")
	resp.Set(41, "POS00001")
	resp.Set(42, "898440153110001")
	resp.Set(49, "156")

	// 组包并解包应答报文
	respPacked, _ := Pack(resp)
	respParsed, _ := Unpack(respPacked)
	PrintMessage(respParsed)

	// 根据应答码输出交易结果
	code := respParsed.Get(39)
	switch code {
	case "00":
		fmt.Println("\n✅ 交易成功")
	case "51":
		fmt.Println("\n❌ 余额不足")
	default:
		fmt.Printf("\n❌ 应答码: %s\n", code)
	}
}
