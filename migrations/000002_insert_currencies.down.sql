DELETE FROM currencies WHERE code IN (
    'USD', 'EUR', 'GBP', 'JPY', 'CNY', 'CAD', 'AUD', 'CHF', 'HKD', 'SGD',
    'SEK', 'NOK', 'PLN', 'DKK', 'MXN', 'NZD', 'CZK', 'HUF', 'ZAR', 'MYR',
    'THB', 'ILS', 'KRW', 'AED', 'SAR', 'BRL', 'INR', 'TRY', 'TWD', 'IDR',
    'PHP', 'CLP', 'COP', 'PEN', 'RON', 'BGN', 'QAR', 'KWD', 'BHD', 'OMR',
    'VND', 'EGP', 'NGN', 'KES', 'MAD', 'PKR', 'BDT', 'KZT', 'ISK', 'ARS'
);
