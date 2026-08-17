#ifndef ERRORINJECTION_H
#define ERRORINJECTION_H


int inject_at_position(char * ,int, int);
void seed_error_injection(void);
void seed_error_injection_value(unsigned int seed);
int last_error_position(void);
int last_error_length(void);
int inject_random_position(char *, int);
int inject_burst(char *, int, int);

#endif 
