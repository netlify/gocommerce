# Gocommerce

A small go based API for static ecommerce sites.

This is a simple order for creating orders with a series of line items, and then
paying for the order with a stripe token.

The API will validate the price of each line item by doing a HTTP request to the
URL for the product and verify a meta tag in this style:

```html
<meta name="product:price:amount" content="4900">
```

The amount must be in cents.

# JavaScript Client Library

The easiest way to use Gocommerce is with [gocommerce-js](https://github.com/netlify/gocommerce-js).
