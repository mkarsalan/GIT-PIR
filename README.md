# GIT-PIR: Private Cloning Protocol for Remote Git Repositories 

This repository contains steps to reproducible results for the paper GIT-PIR: Private Cloning Protocol for Remote Git Repositories.


## Setup

To run the GIT-PIR code, install Go and a C compiler.

To produce the plots, install Python 3, Anaconda, NumPy, Seaborn Library and Matplotlib.



## Reproducing results from the paper

* Get baseline times to clone Tor repositories
```
cd TorProject/
nano clone_repos.sh
``` 
update `path` in clone_repos.sh

```
./clone_repos.sh
```

This would generate `cloning_times.csv` in `TorProject/` directory. This is the baseline time to clone git repositories



* Confirm local Git recognizes PIR protocol. This would replace any version of git present on your system.
```
cd git-master
make install
~/bin/./git clone pir://<username>@192.168.0.240:<path_to>/TorProject/Server_Repositories/gettor-ansible
```

This should allow "pir" as a valid protocol and should not output "invalid protocol". However, since the server side isn't running, the command would still fails.



* Microbenchmarks of SimplePIR for 1GB database:
```
cd pir/
go test TestBenchmarkSimplePirSingle
``` 

This would create `results_simplePIR_benchmarks.csv` in `results/` directory



* Microbenchmarks of SimplePIR for database of varying sizes:
```
cd pir/
go test TestSimplePIR
``` 

This would create `results_simplePIR.csv` in `results/` directory



* Generate results for Tor repositories split into multiDB, ran sequentially:
```
cd pir/
go test TestTorReposSplitIntoMultiDBSequentially
``` 

This would create `results_tor_repos_split_into_multi_db_sequentially.csv` in `results/` directory



* Generate results for Tor repositories split into multiDB, ran sequentially, for varying chunk_sizes:
```
cd pir/
go test TestTorReposSplitIntoMultiDBSequentially
``` 

This would create `results_tor_repos_split_into_multi_db_sequentially.csv` in `results/` directory



* Generate results for cloning 333 Tor repositories split into multiDB, ran in parallel, for single chunk_size:
```
cd pir/
go test TestTorReposSplitIntoMultiDBForSingleChunkSize
``` 

This would create `results_tor_repos_split_into_multiserver_for_single_chunk_size.csv` in `results/` directory



* Generate results for Tor repositories split into multiDB, ran in parallel, for varying chunk_sizes:
```
cd pir/
go test TestTorReposSplitIntoMultiDB
``` 

This would create `results_tor_repos_split_into_multi_db_parallel.csv` in `results/` directory



* Generate results for Tor repositories split into multiDB, for equal number of chunks, and varying chunk_sizes:
```
cd pir/
go test TestTorReposWithEqualNumOfChunks
``` 

This would create `results_tor_repos_with_equal_chunk_size.csv` in `results/` directory


## Generating graphs from the paper

Once the results are generated, run the jupyter notebook `Graphs.ipynb` in `results` directory, to generate the graphs.
