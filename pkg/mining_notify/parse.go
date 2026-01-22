package mining_notify

import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/mrtnetwork/bitcoin/address"
)

// ParsedNotify contains the extracted meaningful fields from mining.notify
type ParsedNotify struct {
	JobID        string
	PrevHash     string // big-endian hex
	Height       uint64 // may be 0 if not found
	ScriptSig    string // hex
	CoinbaseOuts []CoinbaseOutput
	Version      string
	NBits        string
	NTime        string
	NTimeParsed  time.Time // decoded timestamp
	CleanJobs    bool
}

type CoinbaseOutput struct {
	ValueSatoshis uint64
	ValueBTC      float64
	Type          string
	Address       string
}

// Parse takes a raw JSON string of mining.notify and returns parsed data
func Parse(input string) (*ParsedNotify, error) {
	input = strings.TrimSpace(input)

	var raw map[string]interface{}
	if err := json.Unmarshal([]byte(input), &raw); err != nil {
		return nil, fmt.Errorf("invalid mining.notify JSON: %w", err)
	}

	paramsI, ok := raw["params"]
	if !ok {
		return nil, errors.New("missing 'params' field in mining.notify JSON")
	}

	params, ok := paramsI.([]interface{})
	if !ok {
		return nil, errors.New("'params' must be an array")
	}

	if len(params) < 9 {
		return nil, fmt.Errorf("mining.notify params too short: got %d, expected >=9", len(params))
	}

	// Extract and type-assert each parameter
	jobID, ok := params[0].(string)
	if !ok {
		return nil, errors.New("params[0] (job_id) must be a string")
	}

	prevhashLe, ok := params[1].(string)
	if !ok {
		return nil, errors.New("params[1] (prevhash) must be a string")
	}

	coinbasePart1, ok := params[2].(string)
	if !ok {
		return nil, errors.New("params[2] (coinbase_part1) must be a string")
	}

	coinbasePart2, ok := params[3].(string)
	if !ok {
		return nil, errors.New("params[3] (coinbase_part2) must be a string")
	}

	// params[4] is merkle branches array

	version, ok := params[5].(string)
	if !ok {
		return nil, errors.New("params[5] (version) must be a string")
	}

	nbits, ok := params[6].(string)
	if !ok {
		return nil, errors.New("params[6] (nbits) must be a string")
	}

	ntime, ok := params[7].(string)
	if !ok {
		return nil, errors.New("params[7] (ntime) must be a string")
	}

	cleanJobs, ok := params[8].(bool)
	if !ok {
		return nil, errors.New("params[8] (clean_jobs) must be a boolean")
	}

	result := &ParsedNotify{
		JobID:     jobID,
		Version:   version,
		NBits:     nbits,
		NTime:     ntime,
		CleanJobs: cleanJobs,
	}

	// Previous block hash (reverse LE -> BE)
	result.PrevHash = reverseHashHex(prevhashLe)

	// Try to extract height & scriptSig from coinbase parts
	height, scriptSig := extractHeightFromCoinbase(coinbasePart1, coinbasePart2)
	result.Height = height
	result.ScriptSig = scriptSig

	// Try to extract coinbase reward outputs
	outs, err := extractCoinbaseOutputs(coinbasePart1, coinbasePart2)
	if err != nil {
		return nil, fmt.Errorf("failed to extract coinbase outputs: %w", err)
	}
	result.CoinbaseOuts = outs

	// Parse nTime
	if t, err := strconv.ParseUint(result.NTime, 16, 64); err == nil && t > 1e9 {
		result.NTimeParsed = time.Unix(int64(t), 0).UTC()
	}

	return result, nil
}

func reverseHashHex(leHex string) string {
	leHex = strings.TrimSpace(leHex)
	if len(leHex)%2 == 1 {
		leHex = "0" + leHex
	}
	bytes, err := hex.DecodeString(leHex)
	if err != nil {
		return leHex // fallback
	}
	for i, j := 0, len(bytes)-1; i < j; i, j = i+1, j-1 {
		bytes[i], bytes[j] = bytes[j], bytes[i]
	}
	return hex.EncodeToString(bytes)
}

func extractHeightFromCoinbase(part1, part2 string) (uint64, string) {
	if len(part1) < 90 {
		return 0, ""
	}

	// Skip: version(8) + input count(varint, usually 01 -> 1 byte) + prevout(72 bytes total) about 82 chars
	scriptSigStart := 82
	if len(part1) < scriptSigStart+4 {
		return 0, ""
	}

	scriptLenHex := part1[scriptSigStart : scriptSigStart+2]
	scriptLen, err := strconv.ParseUint(scriptLenHex, 16, 32)
	if err != nil || scriptLen == 0 {
		return 0, ""
	}

	// Collect full scriptSig (may span part1 & part2)
	availableInPart1 := len(part1) - scriptSigStart - 2
	scriptSigHex := part1[scriptSigStart+2:]
	if int(scriptLen)*2 > availableInPart1 {
		needed := int(scriptLen)*2 - availableInPart1
		if len(part2) >= needed {
			scriptSigHex += part2[:needed]
		}
	}

	// Now try to read height push
	if len(scriptSigHex) < 4 {
		return 0, scriptSigHex
	}

	pushByte, _ := strconv.ParseUint(scriptSigHex[0:2], 16, 8)
	var heightBytes string

	switch {
	case pushByte >= 1 && pushByte <= 75:
		end := 2 + int(pushByte)*2
		if len(scriptSigHex) >= end {
			heightBytes = scriptSigHex[2:end]
		}

	case pushByte == 0x4c: // OP_PUSHDATA1
		if len(scriptSigHex) >= 6 {
			lenB, _ := strconv.ParseUint(scriptSigHex[2:4], 16, 32)
			end := 4 + int(lenB)*2
			if len(scriptSigHex) >= end {
				heightBytes = scriptSigHex[4:end]
			}
		}

	case pushByte == 0x4d: // OP_PUSHDATA2
		if len(scriptSigHex) >= 8 {
			lenB, _ := strconv.ParseUint(scriptSigHex[2:6], 16, 32)
			end := 6 + int(lenB)*2
			if len(scriptSigHex) >= end {
				heightBytes = scriptSigHex[6:end]
			}
		}
	}

	if heightBytes == "" {
		return 0, scriptSigHex
	}

	// height is little-endian
	heightLE, err := hex.DecodeString(heightBytes)
	if err != nil {
		return 0, scriptSigHex
	}
	var height uint64
	for i := len(heightLE) - 1; i >= 0; i-- {
		height = height<<8 | uint64(heightLE[i])
	}

	return height, scriptSigHex
}

func extractCoinbaseOutputs(part1, part2 string) ([]CoinbaseOutput, error) {
	coinbaseHex := part2 // outputs usually in part2
	if len(coinbaseHex) < 20 {
		return nil, errors.New("coinbase part2 too short")
	}

	var offset int

	// Heuristic: look for ffffffff + output count
	found := false
	for i := 0; i <= len(coinbaseHex)-10; i += 2 {
		if coinbaseHex[i:i+8] == "ffffffff" {
			next, _ := strconv.ParseUint(coinbaseHex[i+8:i+10], 16, 8)
			if next >= 1 && next <= 10 {
				offset = i + 8
				found = true
				break
			}
		}
	}

	if !found {
		if strings.HasPrefix(coinbaseHex, "ffffffff") {
			offset = 8
		} else {
			return nil, errors.New("could not locate outputs start")
		}
	}

	// Skip SegWit marker+flag if present
	if offset+4 <= len(coinbaseHex) && coinbaseHex[offset:offset+2] == "00" {
		flag := coinbaseHex[offset+2 : offset+4]
		if flag == "00" || flag == "01" {
			offset += 4
		}
	}

	outputCount, err := strconv.ParseUint(coinbaseHex[offset:offset+2], 16, 8)
	if err != nil {
		return nil, err
	}
	offset += 2

	var outputs []CoinbaseOutput

	for i := uint64(0); i < outputCount; i++ {
		if offset+16+2 > len(coinbaseHex) {
			break
		}

		// value LE -> BE -> uint64
		valLE := coinbaseHex[offset : offset+16]
		valBytes, _ := hex.DecodeString(valLE)
		for i, j := 0, 7; i < j; i, j = i+1, j-1 {
			valBytes[i], valBytes[j] = valBytes[j], valBytes[i]
		}
		valueSat := binary.BigEndian.Uint64(valBytes)
		offset += 16

		scriptLen, err := strconv.ParseUint(coinbaseHex[offset:offset+2], 16, 32)
		if err != nil {
			break
		}
		offset += 2

		if offset+int(scriptLen)*2 > len(coinbaseHex) {
			break
		}

		script := coinbaseHex[offset : offset+int(scriptLen)*2]
		offset += int(scriptLen) * 2

		out := CoinbaseOutput{
			ValueSatoshis: valueSat,
			ValueBTC:      float64(valueSat) / 1e8,
			Type:          "Unknown",
			Address:       "Unable to decode",
		}

		switch {
		case scriptLen == 25 && strings.HasPrefix(script, "76a914") && strings.HasSuffix(script, "88ac"):
			out.Type = "P2PKH"
			pkh := script[6:46]
			addrObj, err := address.P2PKHAddressFromHash160(pkh)
			if err == nil {
				out.Address = addrObj.Show()
			}
		case scriptLen == 23 && strings.HasPrefix(script, "a914") && strings.HasSuffix(script, "87"):
			out.Type = "P2SH"
			sh := script[4:44]
			addrObj, err := address.P2SHAddressFromHash160(sh)
			if err == nil {
				out.Address = addrObj.Show()
			}
		case scriptLen == 22 && strings.HasPrefix(script, "0014"):
			out.Type = "P2WPKH"
			prog := script[4:]
			addrObj, err := address.P2WPKHAddresssFromProgram(prog)
			if err == nil {
				out.Address = addrObj.Show()
			}
		case scriptLen == 34 && strings.HasPrefix(script, "0020"):
			out.Type = "P2WSH"
			prog := script[4:]
			addrObj, err := address.P2WSHAddresssFromProgram(prog)
			if err == nil {
				out.Address = addrObj.Show()
			}
		case strings.HasPrefix(script, "6a"):
			out.Type = "OP_RETURN"
			out.Address = "(Null Data)"
		}

		outputs = append(outputs, out)
	}

	return outputs, nil
}
