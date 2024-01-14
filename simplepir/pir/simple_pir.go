package pir

// #include "pir.h"
import "C"
import "fmt"
// import "time"

type SimplePIR struct{}

func (pi *SimplePIR) Name() string {
	return "SimplePIR"
}

func (pi *SimplePIR) PickParams(N, d, n, logq uint64) Params {
	good_p := Params{}
	found := false

	// Iteratively refine p and DB dims, until find tight values
	for mod_p := uint64(2); ; mod_p += 1 {
		l, m := ApproxSquareDatabaseDims(N, d, mod_p)

		p := Params{
			N:    n,
			Logq: logq,
			L:    l,
			M:    m,
		}
		p.PickParams(false, m)

		if p.P < mod_p {
			if !found {
				panic("Error; should not happen")
			}
			// good_p.PrintParams() // ==> uncomment to print params
			return good_p
		}

		good_p = p
		found = true
	}

	panic("Cannot be reached")
	return Params{}
}

func (pi *SimplePIR) PickParamsGivenDimensions(l, m, n, logq uint64) Params {
	p := Params{
		N:    n,
                Logq: logq,
                L:    l,
                M:    m,
	}
        p.PickParams(false, m)
        return p
}

// Works for SimplePIR because vertical concatenation doesn't increase
// the number of LWE samples (so don't need to change LWE params)
func (pi *SimplePIR) ConcatDBs(DBs []*Database, p *Params) *Database {
        if len(DBs) == 0 {
                panic("Should not happen")
        }

        if DBs[0].Info.Num != p.L * p.M {
                panic("Not yet implemented")
        }

        rows := DBs[0].Data.Rows
        for j:=1; j<len(DBs); j++ {
                if DBs[j].Data.Rows != rows {
                        panic("Bad input")
                }
        }

        D := new(Database)
        D.Data = MatrixZeros(0, 0)
        D.Info = DBs[0].Info
        D.Info.Num *= uint64(len(DBs))
        p.L *= uint64(len(DBs))

	for j:=0; j<len(DBs); j++ {
		D.Data.Concat(DBs[j].Data.SelectRows(0, rows))
	}

        return D
}

func (pi *SimplePIR) GetBW(info DBinfo, p Params) {
	offline_download := float64(p.L*p.N*p.Logq) / (8.0 * 1024.0)
	fmt.Printf("\t\tOffline download: %d KB\n", uint64(offline_download))

	online_upload := float64(p.M*p.Logq) / (8.0 * 1024.0)
	fmt.Printf("\t\tOnline upload: %d KB\n", uint64(online_upload))

	online_download := float64(p.L*p.Logq) / (8.0 * 1024.0)
	fmt.Printf("\t\tOnline download: %d KB\n", uint64(online_download))
}

func (pi *SimplePIR) Init(info DBinfo, p Params) State {
        A := MatrixRand(p.M, p.N, p.Logq, 0)
        return MakeState(A)
}

func (pi *SimplePIR) InitCompressed(info DBinfo, p Params) (State, CompressedState) {
	seed := RandomPRGKey()
	return pi.InitCompressedSeeded(info, p, seed) 
}

func (pi *SimplePIR) InitCompressedSeeded(info DBinfo, p Params, seed *PRGKey) (State, CompressedState) {
        bufPrgReader = NewBufPRG(NewPRG(seed))
        return pi.Init(info, p), MakeCompressedState(seed)
}

func (pi *SimplePIR) DecompressState(info DBinfo, p Params, comp CompressedState) State {
	bufPrgReader = NewBufPRG(NewPRG(comp.Seed))
	return pi.Init(info, p)
}

func (pi *SimplePIR) Setup(DB *Database, shared State, p Params) (State, Msg) {
	// fmt.Println("==> In Setup")
	A := shared.Data[0]
	// fmt.Println("==> Before MatrixMul")
	H := MatrixMul(DB.Data, A)
	// fmt.Println("==> After MatrixMul")
	// map the database entries to [0, p] (rather than [-p/1, p/2]) and then
	// pack the database more tightly in memory, because the online computation
	// is memory-bandwidth-bound
	DB.Data.Add(p.P / 2)
	DB.Squish()

	return MakeState(), MakeMsg(H)
}

func (pi *SimplePIR) FakeSetup(DB *Database, p Params) (State, float64) {
	offline_download := float64(p.L*p.N*uint64(p.Logq)) / (8.0 * 1024.0)
	fmt.Printf("\t\tOffline download: %d KB\n", uint64(offline_download))

	// map the database entries to [0, p] (rather than [-p/1, p/2]) and then
	// pack the database more tightly in memory, because the online computation
	// is memory-bandwidth-bound
	DB.Data.Add(p.P / 2)
	DB.Squish()

	return MakeState(), offline_download
}

func (pi *SimplePIR) Query(i uint64, shared State, p Params, info DBinfo) (State, Msg) {
	// fmt.Println("\n==> In Query")
	// fmt.Println("==> i:", i)
	A := shared.Data[0]

	secret := MatrixRand(p.N, 1, p.Logq, 0)
	// secret.Dim()
	err := MatrixGaussian(p.M, 1)
	// err.Dim()
	query := MatrixMul(A, secret)
	// query.Dim()
	query.MatrixAdd(err)
	// fmt.Println("==> query.Data x1:", query.Data)
	query.Data[i%p.M] += C.Elem(p.Delta())
	// fmt.Println("==> p.Delta():", C.Elem(p.Delta()))
	// fmt.Println("==> i p.M:", i%p.M)
	// fmt.Println("==> query.Data x2:", query.Data)
	// query.Print()
	// Pad the query to match the dimensions of the compressed DB
	if p.M%info.Squishing != 0 {
		query.AppendZeros(info.Squishing - (p.M % info.Squishing))
	}
	// fmt.Println("==> query.Data x3:", query.Data)
	// query.Print()

	return MakeState(secret), MakeMsg(query)
}

func (pi *SimplePIR) Answer(DB *Database, query MsgSlice, server State, shared State, p Params) Msg {
	ans := new(Matrix)
	num_queries := uint64(len(query.Data)) // number of queries in the batch of queries
	// fmt.Println("==> query.Data:", query.Data);
	batch_sz := DB.Data.Rows / num_queries // how many rows of the database each query in the batch maps to

	last := uint64(0)

	// Run SimplePIR's answer routine for each query in the batch
	for batch, q := range query.Data {
		// fmt.Println("==> batch:", batch);
		// fmt.Println("==> q:", q);

		if batch == int(num_queries-1) {
			batch_sz = DB.Data.Rows - last
		}
		a := MatrixMulVecPacked(DB.Data.SelectRows(last, batch_sz),
			q.Data[0],
			DB.Info.Basis,
			DB.Info.Squishing)
		// fmt.Println("==> a:", a);
		ans.Concat(a)
		last += batch_sz
	}
	// fmt.Println("==> ans:", ans);

	return MakeMsg(ans)
}

func (pi *SimplePIR) Recover(i uint64, batch_index uint64, offline Msg, query Msg, answer Msg,
	shared State, client State, p Params, info DBinfo) uint64 {
	secret := client.Data[0]
	H := offline.Data[0]
	ans := answer.Data[0]

	ratio := p.P/2
	offset := uint64(0);
	for j := uint64(0); j<p.M; j++ {
        	offset += ratio*query.Data[0].Get(j,0)
	}
	offset %= (1 << p.Logq)
	offset = (1 << p.Logq)-offset

	row := i / p.M
	// fmt.Printf("==> (CLIENT): Before MatrixMul")
	interm := MatrixMul(H, secret)
	// fmt.Printf("==> (CLIENT): After MatrixMul")
	ans.MatrixSub(interm)

	var vals []uint64
	// Recover each Z_p element that makes up the desired database entry
	for j := row * info.Ne; j < (row+1)*info.Ne; j++ {
		noised := uint64(ans.Data[j]) + offset
		denoised := p.Round(noised)
		vals = append(vals, denoised)
		//fmt.Printf("Reconstructing row %d: %d\n", j, denoised)
	}
	ans.MatrixAdd(interm)

	return ReconstructElem(vals, i, info)
}

func (pi *SimplePIR) RecoverRepository(i uint64, batch_index uint64, offline Msg, query Msg, answer Msg,
	shared State, client State, p Params, info DBinfo, metadata []uint64) []uint64 {

	secret := client.Data[0]
	H := offline.Data[0]
	ans := answer.Data[0]

	ratio := p.P/2
	offset := uint64(0);
	for j := uint64(0); j<p.M; j++ {
        	offset += ratio*query.Data[0].Get(j,0)
	}

	offset %= (1 << p.Logq)
	offset = (1 << p.Logq)-offset

	row := i / p.M
	interm := MatrixMul(H, secret)
	ans.MatrixSub(interm)

	var vals []uint64
	// Recover each Z_p element that makes up the desired database entry
	for j := row * info.Ne; j < (row+1)*info.Ne; j++ {
		noised := uint64(ans.Data[j]) + offset
		denoised := p.Round(noised)
		vals = append(vals, denoised)
	}

	ans.MatrixAdd(interm)

	q := uint64(1 << info.Logq)

	for i, _ := range vals {
		vals[i] = (vals[i] + info.P/2) % q
		vals[i] = vals[i] % info.P
	}

    	// result_arr := append(metadata, vals...)
	// fmt.Println("==> vals", vals)
	// fmt.Println("==> vals", vals)
	// fmt.Println("==> vals", vals)
	// fmt.Println("==> vals", vals)

	// RESULTS = append(RESULTS, vals)
	// fmt.Println("==> vals:", vals)
	// fmt.Println("==> len(vals):", len(vals))

	// fmt.Println("==> len(RESULTS):", len(RESULTS))
	// convertBytesToRepo(vals, index)
	// convertBytesToRepo(vals[:originalSize], index)


	return vals

	// return ReconstructElemRepo(vals, i, info, metadata)
}

func (pi *SimplePIR) RecoverFile(i uint64, batch_index uint64, offline Msg, query Msg, answer Msg,
	shared State, client State, p Params, info DBinfo, metadata []uint64) uint64 {
	secret := client.Data[0]
	H := offline.Data[0]
	ans := answer.Data[0]

	ratio := p.P/2
	offset := uint64(0);
	for j := uint64(0); j<p.M; j++ {
        	offset += ratio*query.Data[0].Get(j,0)
	}
	offset %= (1 << p.Logq)
	offset = (1 << p.Logq)-offset

	row := i / p.M
	// fmt.Println("==> (CLIENT): Before MatrixMul")
	interm := MatrixMul(H, secret)
	// fmt.Println("==> (CLIENT): After MatrixMul")
	ans.MatrixSub(interm)

	var vals []uint64
	// Recover each Z_p element that makes up the desired database entry
	for j := row * info.Ne; j < (row+1)*info.Ne; j++ {
		noised := uint64(ans.Data[j]) + offset
		denoised := p.Round(noised)
		vals = append(vals, denoised)
		//fmt.Printf("Reconstructing row %d: %d\n", j, denoised)
	}
	ans.MatrixAdd(interm)

	return ReconstructElemRepo(vals, i, info, metadata)
}

func (pi *SimplePIR) Reset(DB *Database, p Params) {
	// Uncompress the database, and map its entries to the range [-p/2, p/2].
	DB.Unsquish()
	DB.Data.Sub(p.P / 2)
}
