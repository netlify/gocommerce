# GoCommerce

A small go based API for static e-commerce sites.

It handles orders and payments. Integrates with Stripe for payments and will support
international pricing and VAT verification.

GoCommerce is released under the [MIT License](LICENSE).
Please make sure you understand its [implications and guarantees](https://writing.kemitchell.com/2016/09/21/MIT-License-Line-by-Line.html).

## Setting up

See [the example configuration](config.example.json) for an example of how to configure
GoCcommerce.

The most important setting is the `site_url`. GoCommerce is always tied to a website,
and will use the site URL to verify product prices, offers, and settings for countries,
product types and VAT rules.

GoCommerce will also look for email templates within a designated site folder and use
the site URL to construct links to order history.

Create a `config.json` file based on `config.example.json` - You must set the `site_url`,
`stripe_key` and paypal `client_id` and `secret` as a minimum.  You can get paypal keys by
creating an app at on the `REST API apps` section of the [paypal developer website](https://developer.paypal.com/developer/applications/).

### What your static site must support

Each product you want to sell from your static site must have unique URL where GoCommerce
can find the meta data needed for calculating pricing and taxes in order to verify that
the order is legitimate before using Stripe to charge the client.

The metadata can be anywhere on the page, and goes in a script tag in this format:

<script id="gocommerce-product" type="application/json">
{"sku": "my-product", "title": "My Product", "prices": [{"amount": "49.99"}], "type": "ebook"}
</script>

The minimum required is the Sku, title and at least one "price". Default currency is USD if nothing else specified.

### Mail templates (Not implemented yet)

GoCommerce will look for mail templates inside `https://yoursite.com/gocommerce/emails/`
when sending mails to users or administrators.

Right now the mail templates are:

* **Order Confirmation** `gocommerce/emails/confirmation.html`

### VAT, Countries and Regions

GoCommerce will regularly check for a file called `https://yoursite.com/gocommerce/settings.json`

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


# JavaScript Client Library

The easiest way to use GoCommerce is with [commerce-js](https://github.com/netlify/netlify-commerce-js).
