# Gocommerce

A small go based API for static e-commerce sites.

It handles orders and payments. Integrates with Stripe for payments and will support
international pricing and VAT verification.

## Setting up

See [the example configuration](config.example.json) for an example of how to configure
Gocommerce.

The most important setting is the `site_url`. Gocommerce is always tied to a website,
and will use the site URL to verify product prices, offers, and settings for countries,
product types and VAT rules.

Gocommerce will also look for email templates within a designated site folder and use
the site URL to construct links to order history.

Create a `config.json` file based on `config.example.json` - You must set the `site_url`
and the `stripe_key` as a minimum.

### What your static site must support

Each product you want to sell from your static site must have unique URL where Gocommerce
can find the meta data needed for calculating pricing and taxes in order to verify that
the order is legitimate before using Stripe to charge the client.

The metadata can be anywhere on the page, and goes in a script tag in this format:

<script id="gocommerce-product" type="application/json">
{"sku": "my-product", "title": "My Product", "prices": [{"amount": "49.99"}], "type": "ebook"}
</script>

More about what the product metadata can contain. The minimum required is the SKU, title and at
least one "price". Default currency is USD if nothing else specified.

### Mail templates

Gocommerce will look for mail templates inside `https://yoursite.com/gocommerce/emails/`
when sending mails to users or administrators.

Right now the mail templates are:

* **Order Confirmation** `gocommerce/emails/confirmation.html`

### VAT, Countries and Regions

Gocommerce will regularly check for a file called `https://yoursite.com/gocommerce/settings.json`

This file should have settings with rules for VAT or currency regions.

This file is not required for gocommerce to work, but if you want to validate shipping/billing
address countries/states and handle VAT calculations or limit certain prices to parts of the
world, you should include one. You can find an example of a real world settings.json in
`examples/settings.json`.

The reason we make you include the file in the static site, is that you'll need to do the same
VAT calculations client side during checkout to be able to show this to the user. The
[gocommerce-js](https://github.com/netlify/gocommerce-js) client library can help you with
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
on the site and the users billing Address is set to "Austria", Gocommerce will verify that a 20 percentage
tax has been included in that product.


# JavaScript Client Library

The easiest way to use Gocommerce is with [gocommerce-js](https://github.com/netlify/gocommerce-js).
