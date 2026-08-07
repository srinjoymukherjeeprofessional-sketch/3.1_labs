#ifndef ERRORINJECTION_H
#define ERRORINJECTION_H


int inject_at_position(char * ,int, int);
void seed_error_injection(void);
int inject_random_position(char *, int);
int inject_burst(char *, int, int);

#endif 
