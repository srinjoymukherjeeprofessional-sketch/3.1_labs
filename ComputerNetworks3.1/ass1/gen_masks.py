def crc16(data):
    crc=0xffff
    for b in data:
        crc ^= b<<8
        for _ in range(8): crc=((crc<<1)^0x1021)&0xffff if crc&0x8000 else (crc<<1)&0xffff
    return crc

def crc10(data):
    crc=0
    for b in data:
        crc ^= b<<2
        for _ in range(8): crc=((crc<<1)^0x233)&0x3ff if crc&0x200 else (crc<<1)&0x3ff
    return crc&0x3ff

def crc8(data):
    crc=0
    for b in data:
        crc ^= b
        for _ in range(8): crc=((crc<<1)^0x07)&0xff if crc&0x80 else (crc<<1)&0xff
    return crc

def crc32(data):
    crc=0xffffffff
    for b in data:
        crc ^= b
        for _ in range(8): crc=((crc>>1)^0xedb88320)&0xffffffff if crc&1 else (crc>>1)&0xffffffff
    return crc^0xffffffff

cases=[
 ('crc8',4,crc8,'msb',[0,6,7,8]),
 ('crc10',3,crc10,'msb',[0,1,5,6,9,10]),
 ('crc16',1,crc16,'msb',[0,4,11,16]),
 ('crc32',2,crc32,'lsb',[0,6,9,10,16,20,21,22,24,25,27,28,30,31,32]),
]

for name,ct,fn,order,bits in cases:
    mask=bytearray(64)
    for q in bits:
        bit=(7-(q%8)) if order=='msb' else (q%8)
        idx=q//8
        mask[16+idx] ^= 1<<bit
    print(f"{name} (type {ct}): ERROR_XOR_MASK_HEX={mask.hex()}")
