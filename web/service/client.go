package service

const sqlInChunk = 500

type ClientService struct{}

func chunkStrings(values []string, size int) [][]string {
	if size <= 0 {
		size = sqlInChunk
	}
	if len(values) == 0 {
		return nil
	}
	chunks := make([][]string, 0, (len(values)+size-1)/size)
	for start := 0; start < len(values); start += size {
		end := start + size
		if end > len(values) {
			end = len(values)
		}
		chunks = append(chunks, values[start:end])
	}
	return chunks
}
