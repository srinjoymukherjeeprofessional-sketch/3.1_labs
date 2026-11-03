# selective repeat
═══ Receiver Summary ════════════════════════
  Mode          : sr
  FCS           : crc32
  Frames accepted : 48
  Frames discarded: 10
  NAKs sent       : 0
  Output file     : output.txt
═════════════════════════════════════════════

═══ Sender Summary ══════════════════════════
  Mode          : sr
  Window        : 4
  FCS           : crc32
  Loss prob     : 0.10
  Corrupt prob  : 0.10
  Total frames  : 87 (data: 48)
  Retransmits   : 39
  Efficiency    : 55.2%
  Total time    : 6.026s
═════════════════════════════════════════════

# Go back n
═══ Receiver Summary ════════════════════════
  Mode          : gbn
  FCS           : crc32
  Frames accepted : 48
  Frames discarded: 18
  NAKs sent       : 0
  Output file     : output.txt
═════════════════════════════════════════════

═══ Sender Summary ══════════════════════════
  Mode          : gbn
  Window        : 4
  FCS           : crc32
  Loss prob     : 0.10
  Corrupt prob  : 0.10
  Total frames  : 82 (data: 48)
  Retransmits   : 34
  Efficiency    : 58.5%
  Total time    : 1.526s
═════════

# stop and wait

═══ Receiver Summary ════════════════════════
  Mode          : stop-wait
  FCS           : crc32
  Frames accepted : 48
  Frames discarded: 8
  NAKs sent       : 0
  Output file     : output.txt
═════════════════════════════════════════════

═══ Sender Summary ══════════════════════════
  Mode          : stop-wait
  Window        : 0
  FCS           : crc32
  Loss prob     : 0.10
  Corrupt prob  : 0.10
  Total frames  : 85 (data: 48)
  Retransmits   : 37
  Efficiency    : 56.5%
  Total time    : 6.969s
═════════════════════════════════════════════


# stop n wait
═══ Receiver Summary ════════════════════════
  Mode          : stop-wait
  FCS           : crc32
  Frames accepted : 48
  Frames discarded: 1
  NAKs sent       : 0
  Output file     : output.txt
═════════════════════════════════════════════
═══ Sender Summary ══════════════════════════
  Mode          : stop-wait
  Window        : 0
  FCS           : crc32
  Loss prob     : 0.30
  Corrupt prob  : 0.01
  Total frames  : 115 (data: 48)
  Retransmits   : 59
  Efficiency    : 41.7%
  Total time    : 21.771s
═════════════════════════════════════════════

# go back n
