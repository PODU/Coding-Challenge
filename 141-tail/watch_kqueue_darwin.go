package main

// oEvtOnly opens a descriptor for event notification only, so watching a file
// does not keep its volume from being unmounted.
const oEvtOnly = 0x8000
