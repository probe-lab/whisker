CREATE TABLE storage_checks
(
    timestamp            DateTime64(3, 'UTC'),             -- when upload probe data was saved
    network              LowCardinality(String),           -- mainnet | testnet
    probe_location       LowCardinality(String),           -- whisker instance location
    run_id               UUID,                             -- identifies the whisker process run
    publisher_url        LowCardinality(String),           -- Walrus publisher endpoint used for upload
    aggregator_url       LowCardinality(String),           -- Walrus aggregator endpoint used for retrieval
    blob_id              String,                           -- base64url blob ID
    sui_object_id        String,                           -- Sui object ID of the uploaded blob
    file_size_bytes      UInt64,                           -- size of the probe file uploaded
    upload_started_at    Nullable(DateTime64(3, 'UTC')),   -- when upload began
    upload_ended_at      Nullable(DateTime64(3, 'UTC')),   -- when upload HTTP request completed
    registered_at        Nullable(DateTime64(3, 'UTC')),   -- timestamp of BlobRegistered Sui event
    certified_at         Nullable(DateTime64(3, 'UTC')),   -- timestamp of BlobCertified Sui event
    retrieval_started_at Nullable(DateTime64(3, 'UTC')),   -- when retrieval request was sent
    retrieval_first_byte_at Nullable(DateTime64(3, 'UTC')), -- when first response byte was received
    retrieval_last_byte_at  Nullable(DateTime64(3, 'UTC')), -- when last response byte was received
    bytes_retrieved      Nullable(UInt64),                 -- bytes received in retrieval response
    status               LowCardinality(String),           -- upload_pending | uploaded | registered | certified | retrieval_pending | retrieved | validated
    content_length_ok    Bool,                             -- retrieved size matches uploaded size
    content_hash_ok      Bool                              -- retrieved content SHA256 matches uploaded
)
ENGINE = MergeTree
PRIMARY KEY (probe_location, timestamp, run_id)
PARTITION BY toStartOfMonth(timestamp)
