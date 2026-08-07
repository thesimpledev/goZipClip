# Where app releases go

Completed release builds that the website serves as downloads are
uploaded to the website's S3 bucket (personal account, us-east-2):

    Bucket: gozipclip.com-825a66c84550b1da
    Prefix: downloads/

Use these exact object keys. The site links to them directly, so the
filenames stay fixed from release to release; each release overwrites
the previous object:

    downloads/zipclip-windows-amd64.zip     (zipped portable .exe)
    downloads/zipclip-linux-amd64.tar.gz    (single binary tarball)

Upload with `s3api put-object`, setting for each object:

    --content-type application/zip          (for the .zip)
    --content-type application/gzip         (for the .tar.gz)
    --cache-control "max-age=3600"

Rules:

- The bucket is public read. Anything placed under `downloads/` is
  world-readable the moment it lands. Never upload debug builds,
  configs, or anything not meant for the public.
- Only write under `downloads/`. Every other key in the bucket is
  owned by the website deploy workflow
  (`.github/workflows/deploy-website.yml`), and its delete pass
  removes unexpected files outside `downloads/`.
- This is separate from the shared `deploy-bucket-b3ae1b2cde6b`
  bin/config convention. That bucket holds private deploy artifacts;
  this prefix holds the public downloads the site links to.
