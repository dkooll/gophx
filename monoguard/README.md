# Requirements

### Input parsing

Parse the data into multiple reports, where each report is represented as a list of integers. Ensure that the input is processed line by line, splitting numbers within each line into individual integers.

### Monotonicity validation

For each report, validate whether the levels follow a consistent monotonic trend:

Strictly increasing: Each number is greater than or equal to the previous one.

Strictly decreasing: Each number is less than or equal to the previous one. If the report switches between increasing and decreasing trends, it is considered invalid.

### Difference validation

Verify that the absolute difference between any two adjacent levels in the report is within the inclusive range of 1 to 3. Reports failing this rule are considered unsafe.

### Counting safe reports
Identify and count all reports that pass both the monotonicity and difference validation criteria. Safe reports must maintain a consistent monotonic trend and have valid differences.

### Output results

Display the total number of safe reports. Optionally, log the status of each report (safe or unsafe) along with the detailed reasoning.
