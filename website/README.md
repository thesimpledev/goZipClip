# Website

Static site for ZipClip (gozipclip.com), serving the Linux and Windows
downloads of the desktop application. Same deployment pattern as
simplyauto.dev: S3 static website endpoint behind CloudFront, ACM
certificate in us-east-1, Route53 for DNS. Personal AWS account,
us-east-2.

## Deployed resources

- Bucket: `gozipclip.com-825a66c84550b1da` (us-east-2, public read,
  static website hosting, index `index.html`, error `404.html`)
- Website endpoint:
  http://gozipclip.com-825a66c84550b1da.s3-website.us-east-2.amazonaws.com/

## Not yet done

- Register gozipclip.com (registrar decision pending)
- Route53 hosted zone, then NS delegation at the registrar
- ACM certificate in us-east-1 for gozipclip.com + www
- CloudFront distribution (http-only custom origin pointing at the
  website endpoint, redirect-to-https, CachingOptimized, compress)
- A/AAAA alias records to the distribution
- Real download links once the app ships

## Deploying changes

Push a `web-v*` tag. The GitHub Actions workflow
(`.github/workflows/deploy-website.yml`) assumes the
`zipclip-website-deploy` IAM role via OIDC and syncs `website/` to the
bucket in phases: assets first (`max-age=86400`), then HTML
(`max-age=300`), then a CloudFront invalidation (once the distribution
exists), then a delete pass that spares `downloads/`. CSS and images
use a `?v=N` query bump in the HTML when they change. Repo variables
needed: `WEBSITE_DEPLOY_ROLE_ARN`, `PROD_S3_BUCKET`, and later
`PROD_CLOUDFRONT_DISTRIBUTION_ID`.

App release binaries are uploaded separately under `downloads/`, see
`DOWNLOADS.md`.

For a manual one-off upload use `s3api put-object` (aws-sec refuses
`s3 cp`), setting `--content-type` and `--cache-control` per file.

`images/21.png` is the 5000px source art (Go gopher, calendar). It is
not uploaded; `hero.png`, `logo.png`, and `favicon.png` are derived
from it with ImageMagick.
