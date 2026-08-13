#ifndef ERRORDETECTION_H
#define ERRORDETECTION_H

unsigned short checksum(const char *, int);
unsigned short crc16(const char *, int);
unsigned short crc10(const char *, int);
unsigned int crc32(const char *, int);

#endif // ERRORDETECTION_H
