package main

type opcode uint8

const (
	OpGET opcode = iota
	OpPUT
	OpVGET
	OpVPUT

	OpLS
	OpMKDIR
	OpRM
	OpMV

	OpRESET
	OpBOOT
	OpPOWER_CYCLE
	OpINFO
	OpMENU_RESET
	OpSTREAM
	OpTIME
	OpRESPONSE
)

type space uint8

const (
	SpaceFILE space = iota
	SpaceSNES
	SpaceMSU
	SpaceCMD
	SpaceCONFIG
)

type server_flags uint8

const FlagNONE server_flags = 0
const (
	FlagSKIPRESET server_flags = 1 << iota
	FlagONLYRESET
	FlagCLRX
	FlagSETX
	FlagSTREAM_BURST
	_
	FlagNORESP
	FlagDATA64B
)

type info_flags uint8

const (
	FeatDSPX info_flags = 1 << iota
	FeatST0010
	FeatSRTC
	FeatMSU1
	Feat213F
	FeatCMD_UNLOCK
	FeatUSB1
	FeatDMA1
)

type file_type uint8

const (
	FtDIRECTORY file_type = 0
	FtFILE      file_type = 1
)

func makeVGET(addr uint32, size uint8) []byte {
	sb := make([]byte, 64)
	sb[0] = byte('U')
	sb[1] = byte('S')
	sb[2] = byte('B')
	sb[3] = byte('A')
	sb[4] = byte(OpVGET)
	sb[5] = byte(SpaceSNES)
	sb[6] = byte(FlagDATA64B | FlagNORESP)
	// 4-byte struct: 1 byte size, 3 byte address
	sb[32] = byte(size)
	sb[33] = byte((addr >> 16) & 0xFF)
	sb[34] = byte((addr >> 8) & 0xFF)
	sb[35] = byte((addr >> 0) & 0xFF)
	return sb
}

func makeGET(addr uint32, size uint32) []byte {
	sb := make([]byte, 512)
	sb[0] = byte('U')
	sb[1] = byte('S')
	sb[2] = byte('B')
	sb[3] = byte('A')
	sb[4] = byte(OpGET)
	sb[5] = byte(SpaceSNES)
	sb[6] = byte(FlagNONE)
	// size:
	sb[252] = byte((size >> 24) & 0xFF)
	sb[253] = byte((size >> 16) & 0xFF)
	sb[254] = byte((size >> 8) & 0xFF)
	sb[255] = byte((size >> 0) & 0xFF)
	// addr:
	sb[256] = byte((addr >> 24) & 0xFF)
	sb[257] = byte((addr >> 16) & 0xFF)
	sb[258] = byte((addr >> 8) & 0xFF)
	sb[259] = byte((addr >> 0) & 0xFF)
	return sb
}
