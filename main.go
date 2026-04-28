package main

import (
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
)

type LenType int

const (
	FIXED  LenType = iota
	LLVAR
	LLLVAR
)

type FieldDef struct {
	ID     int
	Name   string
	Type   LenType
	MaxLen int
}

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

type ISO8583Message struct {
	MTI    string
	Fields map[int]string
}

func NewMessage(mti string) *ISO8583Message {
	return &ISO8583Message{MTI: mti, Fields: make(map[int]string)}
}

func (m *ISO8583Message) Set(fieldID int, value string) { m.Fields[fieldID] = value }
func (m *ISO8583Message) Get(fieldID int) string         { return m.Fields[fieldID] }

func buildBitmap(fields map[int]string) string {
	bitmap := make([]byte, 8)
	for fid := range fields {
		if fid >= 1 && fid <= 64 {
			bitmap[(fid-1)/8] |= 1 << (7 - uint((fid-1)%8))
		}
	}
	return strings.ToUpper(hex.EncodeToString(bitmap))
}

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

func Pack(msg *ISO8583Message) (string, error) {
	var buf strings.Builder
	buf.WriteString(msg.MTI)
	bitmapHex := buildBitmap(msg.Fields)
	buf.WriteString(bitmapHex)
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
			padded := fmt.Sprintf("%-*s", def.MaxLen, value)
			buf.WriteString(padded[:def.MaxLen])
		case LLVAR:
			buf.WriteString(fmt.Sprintf("%02d", len(value)))
			buf.WriteString(value)
		case LLLVAR:
			buf.WriteString(fmt.Sprintf("%03d", len(value)))
			buf.WriteString(value)
		}
	}
	return buf.String(), nil
}

func Unpack(raw string) (*ISO8583Message, error) {
	msg := &ISO8583Message{Fields: make(map[int]string)}
	pos := 0
	if len(raw) < 4 {
		return nil, fmt.Errorf("message too short")
	}
	msg.MTI = raw[pos : pos+4]
	pos += 4
	if len(raw) < pos+16 {
		return nil, fmt.Errorf("message too short for bitmap")
	}
	bitmapHex := raw[pos : pos+16]
	pos += 16
	fields, err := parseBitmap(bitmapHex)
	if err != nil {
		return nil, err
	}
	for _, fid := range fields {
		def, exists := spec[fid]
		if !exists {
			return nil, fmt.Errorf("field %d not defined in spec", fid)
		}
		switch def.Type {
		case FIXED:
			if pos+def.MaxLen > len(raw) {
				return nil, fmt.Errorf("field %d: not enough data", fid)
			}
			msg.Fields[fid] = strings.TrimRight(raw[pos:pos+def.MaxLen], " ")
			pos += def.MaxLen
		case LLVAR:
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

	packed, err := Pack(req)
	if err != nil {
		fmt.Println("组包错误:", err)
		return
	}
	fmt.Printf("报文(%d bytes): %s\n\n", len(packed), packed)
	PrintMessage(req)

	fmt.Println("\n====== 解析报文 ======\n")
	parsed, err := Unpack(packed)
	if err != nil {
		fmt.Println("解包错误:", err)
		return
	}
	PrintMessage(parsed)

	fmt.Println("\n====== 交易应答 (0210) ======\n")
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

	respPacked, _ := Pack(resp)
	respParsed, _ := Unpack(respPacked)
	PrintMessage(respParsed)

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