# GoCommerce [![Build Status](https://travis-ci.org/netlify/gocommerce.svg?branch=master)](https://travis-ci.org/netlify/gocommerce)

A small go based API for static e-commerce sites.

It handles orders and payments. Integrates with Stripe for payments and will support
international pricing and VAT verification.

GoCommerce is released under the [MIT License](LICENSE).
Please make sure you understand its [implications and guarantees](https://writing.kemitchell.com/2016/09/21/MIT-License-Line-by-Line.html).

### What your static site must support

Each product you want to sell from your static site must have unique URL where GoCommerce
can find the meta data needed for calculating pricing and taxes in order to verify that
the order is legitimate before using Stripe to charge the client.

The metadata can be anywhere on the page, and goes in a script tag in this format:

```html
<script class="gocommerce-product" type="application/json">
{"sku": "my-product", "title": "My Product", "prices": [{"amount": "49.99", "currency": "USD"}], "type": "ebook"}
</script>
```

The minimum required is the Sku, title and at least one "price". Default currency is USD if nothing else specified.

### VAT, Countries and Regions

GoCommerce will regularly check for a file called `https://example.com/gocommerce/settings.json`

This file should have settings with rules for VAT or currency regions.

This file is not required for GoCommerce to work, but will enable support for various advanced
features. Currently it enables VAT calculations on a per country/product type basic.

The reason we make you include the file in the static site, is that you'll need to do the same
VAT calculations client side during checkout to be able to show this to the user. The
[commerce-js](https://github.com/netlify/netlify-commerce-js) client library can help you with
this.

Here's an example settings file:

```json
{
  "taxes": [{
    "percentage": 20,
    "product_types": ["ebook"],
    "countries": ["Austria", "Bulgaria", "Estonia", "France", "Gibraltar", "Slovakia", "United Kingdom"]
  }, {
    "percentage": 7,
    "product_types": ["book"],
    "countries": ["Austria", "Belgium", "Bulgaria", "Croatia", "Cyprus", "Denmark", "Estonia"]
  }]
}
```

Based on these rules, if an order includes a product with "type" set to "ebook" in the product metadata
on the site and the users billing Address is set to "Austria", GoCommerce will verify that a 20 percentage
tax has been included in that product.


## JavaScript Client Library

The easiest way to use GoCommerce is with [commerce-js](https://github.com/netlify/netlify-commerce-js).

## Running the GoCommerce backend

GoCommerce can be deployed to any server environment that runs Go. The button below provides a quick way to get started by running on Heroku:

[![Deploy](https://www.herokucdn.com/deploy/button.svg)](https://heroku.com/deploy?template=https://github.com/netlify/gocommerce)

## Configuration

You may configure GoCommerce using either a configuration file named `.env`,
environment variables, or a combination of both. Environment variables are prefixed with `GOCOMMERCE_`, and will always have precedence over values provided via file.

For local dev, the easiest way to get started is to copy the included `example.env` file to `.env`

### Top-Level

```
GOCOMMERCE_SITE_URL=https://example.netlify.com/
```

`SITE_URL` - `string` **required**

The base URL your site is located at.

`OPERATOR_TOKEN` - `string` *Multi-instance mode only*

The shared secret with an operator (usually Netlify) for this microservice. Used to verify requests have been proxied through the operator and
the payload values can be trusted.

### API

```
GOCOMMERCE_API_HOST=localhost
PORT=9999
```

`API_HOST` - `string`

Hostname to listen on.

`PORT` (no prefix) / `API_PORT` - `number`

Port number to listen on. Defaults to `8080`.

`API_ENDPOINT` - `string` *Multi-instance mode only*

Controls what endpoint Netlify can access this API on.

### Database

```
GOCOMMERCE_DB_DRIVER=sqlite3
DATABASE_URL=gotrue.db
```

`DB_DRIVER` - `string` **required**

Chooses what dialect of database you want. Choose from `sqlite3`, `mysql`, or `postgres`.

`DATABASE_URL` (no prefix) / `DB_DATABASE_URL` - `string` **required**

Connection string for the database. See the [gorm examples](https://github.com/jinzhu/gorm/blob/gh-pages/documents/database.md) for more details.

`DB_NAMESPACE` - `string`

Adds a prefix to all table names.

`DB_AUTOMIGRATE` - `bool`

If enabled, creates missing tables and columns upon startup.

### Logging

```
LOG_LEVEL=debug
```

`LOG_LEVEL` - `string`

Controls what log levels are output. Choose from `panic`, `fatal`, `error`, `warn`, `info`, or `debug`. Defaults to `info`.

`LOG_FILE` - `string`

If you wish logs to be written to a file, set `log_file` to a valid file path.

### Payment

#### Stripe

`PAYMENT_STRIPE_ENABLED` - `bool`

Whether Stripe is enabled as a payment provider or not.

`PAYMENT_STRIPE_SECRET_KEY` - `string`

The Stripe [secret key](https://stripe.com/docs/api#authentication) used when authenticating with the Stripe API.

#### PayPal

`PAYMENT_PAYPAL_ENABLED` - `bool`

Whether PayPal is enabled as a payment provider or not.

`PAYMENT_PAYPAL_CLIENT_ID` - `string`
`PAYMENT_PAYPAL_SECRET` - `string`

The OAuth credentials PayPal issued to you. GoCommerce will use them to [obtain an access token](https://developer.paypal.com/docs/api/overview/#authentication-and-authorization).

`PAYMENT_PAYPAL_ENV` - `string`

The PayPal environment to use. Choose from `production` or `sandbox`.

### Downloads

`DOWNLOADS_PROVIDER` - `string`

The provider to use for downloads. Choose from `netlify` or ``.

`DOWNLOADS_NETLIFY_TOKEN` - `string`

The authentication bearer token used to access the Netlify downloads API.

### Coupons

`COUPONS_URL` - `string`

A URL that contains all the coupon information in JSON.

`COUPONS_USER` - `string`
`COUPONS_PASSWORD` - `string`

HTTP Basic Authentication information to use if required to access the coupon information.

### Webhooks

`WEBHOOKS_ORDER` - `string`
`WEBHOOKS_PAYMENT` - `string`
`WEBHOOKS_UPDATE` - `string`
`WEBHOOKS_REFUND` - `string`

A URL to send a webhook to when the corresponding action has been performed.

`WEBHOOKS_SECRET` - `string`

A secret used to sign a JWT included in the `X-Commerce-Signature` header. This can be used to verify the webhook came from GoCommerce.

### JSON Web Tokens (JWT)

```
GOCOMMERCE_JWT_SECRET=supersecretvalue
```

`JWT_SECRET` - `string` **required**

The secret used to verify JWT tokens with.

`JWT_ADMIN_GROUP_NAME` - `string`

The name of the admin group (if enabled). Defaults to `admin`.

### E-Mail

Sending email is not required, but is highly recommended.
If enabled, you must provide the required values below.

```
GOCOMMERCE_SMTP_HOST=smtp.mandrillapp.com
GOCOMMERCE_SMTP_PORT=587
GOCOMMERCE_SMTP_USER=smtp-delivery@example.com
GOCOMMERCE_SMTP_PASS=correcthorsebatterystaple
GOCOMMERCE_SMTP_ADMIN_EMAIL=support@example.com
GOCOMMERCE_MAILER_SUBJECTS_ORDER_CONFIRMATION="Please confirm"
```

`SMTP_ADMIN_EMAIL` - `string` **required**

The `From` email address for all emails sent. Order receipts are also sent to this address.

`SMTP_HOST` - `string` **required**

The mail server hostname to send emails through.

`SMTP_PORT` - `number` **required**

The port number to connect to the mail server on.

`SMTP_USER` - `string`

If the mail server requires authentication, the username to use.

`SMTP_PASS` - `string`

If the mail server requires authentication, the password to use.

`MAILER_SUBJECTS_ORDER_CONFIRMATION` - `string`

Email subject to use for order confirmations. Defaults to `Order Confirmation`.

`MAILER_SUBJECTS_ORDER_RECEIVED` - `string`

Email subject to use for orders sent to the store admin. Defaults to `Order Received From {{ .Order.Email }}`.

`MAILER_TEMPLATES_ORDER_CONFIRMATION` - `string`

URL path, relative to the `SITE_URL`, of an email template to use when sending an order confirmation.
`Order` and `Transaction` variables are available.

Default Content (if template is unavailable):
```html
<h2>Thank you for your order!</h2>

<ul>
{{ range .Order.LineItems }}
<li>{{ .Title }} <strong>{{ .Quantity }} x {{ .Price }}</strong></li>
{{ end }}
</ul>

<p>Total amount: <strong>{{ .Order.Total }}</strong></p>
```

`MAILER_TEMPLATES_ORDER_RECEIVED` - `string`

URL path, relative to the `SITE_URL`, of an email template to use when sending order details to the store admin.
`Order` and `Transaction` variables are available.

Default Content (if template is unavailable):
```html
<h2>Order Received From {{ .Order.Email }}</h2>

<ul>
{{ range .Order.LineItems }}
<li>{{ .Title }} <strong>{{ .Quantity }} x {{ .Price }}</strong></li>
{{ end }}
</ul>

<p>Total amount: <strong>{{ .Order.Total }}</strong></p>
```
