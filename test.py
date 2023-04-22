from selenium import webdriver

browser = webdriver.Edge(
    executable_path='msedgedriver.exe')
browser.get('http://www.baidu.com/')

browser.execute_script('alert("To Bottom")')
browser.create_options().add_argument('--headless')
# 暂停
input('Press any key to continue...')