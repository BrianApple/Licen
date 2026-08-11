package machine

import "testing"

func TestComputeAnchorPriority(t *testing.T) {
	cases := []struct {
		name string
		mb   string
		cpu  string
		mac  string
		disk string
		uuid string
		salt string
		want string // 期望的锚点（用于断言优先级）
	}{
		{"全部有值→UUID优先", "MB1", "CPU1", "AA:BB", "DISK1", "UUID-1", "", "UUID-1"},
		{"无UUID→磁盘优先", "MB1", "CPU1", "AA:BB", "DISK1", "", "", "DISK1"},
		{"无UUID无磁盘→MAC", "MB1", "CPU1", "AA:BB", "", "", "", "AA:BB"},
		{"仅主板", "MB1", "", "", "", "", "", "MB1"},
		{"仅CPU(退化型号)", "", "i5-8cores", "", "", "", "", "i5-8cores"},
		{"全部为空→空串", "", "", "", "", "", "", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := compute(c.mb, c.cpu, c.mac, c.disk, c.uuid, c.salt)
			if c.want == "" {
				if got != "" {
					t.Fatalf("全部源为空应返回空串，得到 %s", got)
				}
				return
			}
			if got == "" {
				t.Fatal("非空锚点不应返回空串")
			}
			// 校验哈希确实基于锚点：直接算期望值对比
			wantHash := sha256Of(c.want, c.salt)
			if got != wantHash {
				t.Fatalf("机器码不匹配：got=%s want=%s（锚点=%s）", got, wantHash, c.want)
			}
		})
	}
}

func TestComputeConsistency(t *testing.T) {
	// 一致性：UUID 存在时，加盘/换网卡/调vCPU/换主板 都不改变机器码
	base := compute("MB1", "i5-8cores", "AA:BB", "DISK1", "UUID-1", "")
	addDisk := compute("MB1", "i5-8cores", "AA:BB", "DISK2", "UUID-1", "")
	changeNIC := compute("MB1", "i5-8cores", "CC:DD", "DISK1", "UUID-1", "")
	changeCPU := compute("MB1", "i7-16cores", "AA:BB", "DISK1", "UUID-1", "")
	changeMB := compute("MB2", "i5-8cores", "AA:BB", "DISK1", "UUID-1", "")
	for name, got := range map[string]string{
		"加盘":   addDisk,
		"换网卡": changeNIC,
		"调vCPU": changeCPU,
		"换主板": changeMB,
	} {
		if got != base {
			t.Fatalf("一致性破坏：%s 改变了机器码 base=%s got=%s", name, base, got)
		}
	}
	// 一致性：无 UUID 时锚点=磁盘，换网卡/调vCPU 不变
	base2 := compute("MB1", "i5-8cores", "AA:BB", "DISK1", "", "")
	if got := compute("MB1", "i7-16cores", "AA:BB", "DISK1", "", ""); got != base2 {
		t.Fatalf("无UUID场景调vCPU不应改变机器码 base=%s got=%s", base2, got)
	}
	// 盐参与哈希：同锚点不同盐 → 不同码
	if compute("", "", "", "", "UUID-1", "s1") == compute("", "", "", "", "UUID-1", "s2") {
		t.Fatal("不同盐应产生不同机器码")
	}
	// 确定性：同输入两次计算一致
	if compute("MB1", "i5", "AA", "D1", "U1", "s") != compute("MB1", "i5", "AA", "D1", "U1", "s") {
		t.Fatal("同输入应产生相同机器码（确定性）")
	}
}

func sha256Of(anchor, salt string) string {
	// 与 compute 内哈希逻辑一致（供测试断言）
	return compute(anchor, "", "", "", "", salt)
}
