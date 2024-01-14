# GIT-PIR: Private Cloning Protocol for Remote Git Repositories 

This repository contains steps to reproducible results for the paper GIT-PIR: Private Cloning Protocol for Remote Git Repositories.


## Setup

To run the GIT-PIR code, install Go and a C compiler.

To produce the plots, install Python 3, NumPy and Matplotlib.



## Reproducing results from the paper

* Table 1:
```
cd pir/
go test Table_1
cd ../results
python3 table_1.py
``` 

This would create `table_1.csv` in `results/` directory


* Table 2:
```
cd pir/
go test Table_2
cd ../results
python3 table_2.py
``` 

This would create `table_2.csv` in `results/` directory


* Figure 8 and 9:
```
cd pir/
go test Figure_8_9
cd ../results
python3 figure_8_9.py
``` 

This would create `figure_8.png` and `figure_9.png` in `results/` directory


* Figure 12:
```
cd pir/
go test Figure_12
cd ../results
python3 figure_12.py
``` 

This would create `figure_12.png` in `results/` directory


* Figure 10, 11 and 13:
```
cd pir/
go test Figure_10_11_13
cd ../results
python3 figure_10_11_13.py
``` 

This would create `figure_10.png`, `figure_11.png` and `figure_13.png` in `results/` directory

